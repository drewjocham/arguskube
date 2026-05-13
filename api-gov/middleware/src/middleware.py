"""FastAPI middleware that pushes OpenAPI specs and traffic to API Gov.

Design notes
------------

Body capture
    FastAPI/Starlette consumes the request body via an async ASGI stream. Once
    ``await request.body()`` (or ``request.json()``) is awaited, the stream is
    drained and downstream route handlers receive an empty body. We avoid that
    by reading the body once, caching it on ``request.state`` AND repackaging
    it back onto the receive channel so the inner app sees the bytes again.

Fire-and-forget
    Traffic ingestion must never block the response. We schedule it on a
    bounded background queue; if the queue is full we drop the sample and
    increment a counter rather than waiting.

Lifespan
    We don't monkey-patch ``app.router.lifespan``. We expose an
    ``async_context``-style start/stop pair plus a public ``setup_lifespan``
    helper that the caller composes with their own lifespan handler.

Dependency injection
    The :class:`httpx.AsyncClient` is injected so tests can pass a mock
    transport. The middleware never instantiates the client implicitly unless
    no other client has been supplied.
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
import os
import random
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager
from typing import Any

import httpx
from fastapi import FastAPI, Request, Response
from opentelemetry import trace
from pydantic import BaseModel
from starlette.middleware.base import BaseHTTPMiddleware

logger = logging.getLogger(__name__)
tracer = trace.get_tracer("api-gov-middleware")

# Hop-by-hop headers per RFC 7230 §6.1 — these describe a single TCP hop and
# must never be propagated. Capturing them as "API metadata" is meaningless
# and may leak proxy details. Stored lowercase so comparison is cheap.
_HOP_BY_HOP_HEADERS = frozenset(
    {
        "connection",
        "keep-alive",
        "proxy-authenticate",
        "proxy-authorization",
        "te",
        "trailer",
        "transfer-encoding",
        "upgrade",
        # Authorization is *meaningful* but sensitive — strip it from
        # captured metadata to avoid landing bearer tokens in the gov DB.
        "authorization",
        "cookie",
        "set-cookie",
    }
)

# Bound on the in-process traffic queue. The middleware drops samples (with a
# warning every Nth drop) instead of growing memory unboundedly.
_DEFAULT_QUEUE_SIZE = 256


class APIGovConfig(BaseModel):
    """Static configuration for the middleware.

    All fields can be overridden by environment variables — see
    :func:`APIGovConfig.from_env`.
    """

    api_url: str = "http://localhost:8080"
    spec_id: str = ""
    api_key: str = ""
    auto_ingest_traffic: bool = True
    sample_rate: float = 1.0  # 0.0–1.0
    retry_max: int = 3
    max_body_bytes: int = 1 * 1024 * 1024  # 1 MiB — anything larger is dropped
    queue_size: int = _DEFAULT_QUEUE_SIZE
    http_timeout_seconds: float = 10.0
    # Batching: the ingest worker collects up to ``batch_max_size`` samples
    # OR waits up to ``batch_max_wait_ms`` past the first queued sample
    # before flushing them in a single POST to /api/v1/traffic/batch.
    # Both bounds protect different failure modes:
    #   * batch_max_size keeps memory + payload predictable under burst
    #   * batch_max_wait_ms keeps the per-sample latency bounded under
    #     low traffic (a single rare sample shouldn't sit forever)
    batch_max_size: int = 50
    batch_max_wait_ms: int = 500

    @classmethod
    def from_env(cls) -> "APIGovConfig":
        """Build a config from API_GOV_* environment variables."""
        return cls(
            api_url=os.getenv("API_GOV_URL", "http://localhost:8080"),
            spec_id=os.getenv("API_GOV_SPEC_ID", ""),
            api_key=os.getenv("API_GOV_API_KEY", ""),
            sample_rate=float(os.getenv("API_GOV_SAMPLE_RATE", "1.0")),
            max_body_bytes=int(os.getenv("API_GOV_MAX_BODY_BYTES", str(1024 * 1024))),
            batch_max_size=int(os.getenv("API_GOV_BATCH_MAX_SIZE", "50")),
            batch_max_wait_ms=int(os.getenv("API_GOV_BATCH_MAX_WAIT_MS", "500")),
        )


# Type alias for the receive channel reinstaller — see _cache_body.
_Receive = Callable[[], Awaitable[dict[str, Any]]]


class APIGovMiddleware(BaseHTTPMiddleware):
    """ASGI middleware that uploads specs and per-request traffic samples.

    This subclasses :class:`BaseHTTPMiddleware` so it integrates with FastAPI's
    standard ``app.add_middleware`` API — the previous implementation wrapped
    the app via ``app.middleware("http")`` and tweaked ``router.lifespan``,
    both of which are brittle private surfaces.
    """

    def __init__(
        self,
        app: FastAPI,
        config: APIGovConfig | None = None,
        *,
        http_client: httpx.AsyncClient | None = None,
    ) -> None:
        super().__init__(app)
        self._fastapi_app = app
        self.config = config or APIGovConfig.from_env()

        # Use the caller's client when supplied (the test suite passes a
        # mock-transport client). Otherwise build one from the config.
        self._owns_client = http_client is None
        self._client: httpx.AsyncClient = http_client or httpx.AsyncClient(
            base_url=self.config.api_url,
            timeout=self.config.http_timeout_seconds,
        )

        # The background ingest queue + worker are created lazily on startup.
        self._queue: asyncio.Queue[dict[str, Any]] | None = None
        self._worker_task: asyncio.Task[None] | None = None
        # Track dropped samples so an operator can see the middleware fell
        # behind. Logged every 100 drops to avoid log spam.
        self._dropped = 0

    # ------------------------------------------------------------------
    # Lifespan composition — replaces the router-mutation hack.
    # ------------------------------------------------------------------

    @asynccontextmanager
    async def lifespan(self, app: FastAPI) -> AsyncIterator[None]:
        """Lifespan context manager. Compose with the caller's lifespan:

            @asynccontextmanager
            async def app_lifespan(app):
                async with middleware.lifespan(app):
                    # ... your own startup ...
                    yield

        Or use :func:`setup_lifespan` for the common case.
        """
        await self._startup()
        try:
            yield
        finally:
            await self._shutdown()

    async def _startup(self) -> None:
        """Push the OpenAPI spec, start the ingest worker."""
        await self._push_spec()
        self._queue = asyncio.Queue(maxsize=self.config.queue_size)
        self._worker_task = asyncio.create_task(self._ingest_worker(), name="api-gov-ingest")

    async def _shutdown(self) -> None:
        """Drain any remaining samples, then close the http client.

        We give the worker a short, bounded window to flush its queue
        before cancelling — otherwise samples already in flight from
        the user's last requests would be dropped at process exit. The
        cap is intentionally small so a hung backend can't stall app
        shutdown for the full ``http_timeout_seconds``.
        """
        if self._queue is not None and self._worker_task is not None:
            drain_budget = max(
                0.25,
                (self.config.batch_max_wait_ms / 1000.0) * 2.0,
            )
            try:
                await asyncio.wait_for(self._queue.join(), timeout=drain_budget)
            except asyncio.TimeoutError:
                logger.info(
                    "api-gov: shutdown drain timed out; %d samples may be dropped",
                    self._queue.qsize(),
                )

        if self._worker_task is not None:
            self._worker_task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await self._worker_task
            self._worker_task = None
        if self._owns_client:
            await self._client.aclose()

    # ------------------------------------------------------------------
    # Spec push (one-shot at startup, retried).
    # ------------------------------------------------------------------

    async def _push_spec(self) -> None:
        try:
            openapi = self._fastapi_app.openapi()
        except (RuntimeError, AttributeError) as e:
            logger.warning("OpenAPI generation failed: %s", e)
            return
        if not openapi:
            logger.warning("no OpenAPI spec found on app")
            return

        name = openapi.get("info", {}).get("title", "unknown")
        version = openapi.get("info", {}).get("version", "1.0.0")
        payload = {"name": name, "version": version, "content": openapi}

        for attempt in range(1, self.config.retry_max + 1):
            try:
                resp = await self._client.post(
                    "/api/v1/specs",
                    json=payload,
                    headers=self._auth_headers(),
                )
            except httpx.RequestError as e:
                logger.warning(
                    "api-gov spec push attempt %d/%d: connection error: %s",
                    attempt,
                    self.config.retry_max,
                    e,
                )
                continue

            if resp.is_success:
                try:
                    data = resp.json()
                except ValueError:
                    logger.warning("api-gov returned non-JSON success body; spec_id not captured")
                    return
                self.config.spec_id = data.get("id", "") if isinstance(data, dict) else ""
                logger.info("api-gov spec pushed: %s v%s (id=%s)", name, version, self.config.spec_id)
                return

            logger.warning(
                "api-gov spec push attempt %d/%d: HTTP %d",
                attempt,
                self.config.retry_max,
                resp.status_code,
            )

        logger.error("api-gov spec push failed after %d attempts", self.config.retry_max)

    # ------------------------------------------------------------------
    # Per-request flow.
    # ------------------------------------------------------------------

    async def dispatch(
        self,
        request: Request,
        call_next: Callable[[Request], Awaitable[Response]],
    ) -> Response:
        # Cache the body BEFORE the inner app reads it. This is the only safe
        # place — once call_next runs, the stream is gone.
        body_bytes = await self._cache_body(request)

        response = await call_next(request)

        if self.config.auto_ingest_traffic and self.config.spec_id:
            self._enqueue_sample(request, response, body_bytes)
        return response

    async def _cache_body(self, request: Request) -> bytes | None:
        """Read the body, cap it at ``max_body_bytes``, and re-arm the receive
        channel so the downstream app reads the same bytes.

        Returns the cached bytes (or ``None`` for methods that don't carry a
        body). Stores them on ``request.state.gov_body`` for any route handler
        that wants to peek.
        """
        if request.method not in ("POST", "PUT", "PATCH"):
            return None

        # Reject before allocating — Content-Length is cheap and authoritative
        # when present.
        content_length = request.headers.get("content-length")
        if content_length is not None:
            try:
                declared = int(content_length)
            except ValueError:
                declared = -1
            if declared > self.config.max_body_bytes:
                logger.info(
                    "api-gov: skipping body capture for %s %s (Content-Length=%d > max=%d)",
                    request.method,
                    request.url.path,
                    declared,
                    self.config.max_body_bytes,
                )
                # Still rebuild receive so the inner handler can read the
                # stream itself; we just don't capture it.
                return await self._passthrough_body(request)

        try:
            body = await request.body()
        except Exception as e:  # pragma: no cover - defensive
            logger.debug("api-gov: failed to read request body: %s", e)
            return None

        if len(body) > self.config.max_body_bytes:
            logger.info(
                "api-gov: truncating body capture for %s %s (read %d > max=%d)",
                request.method,
                request.url.path,
                len(body),
                self.config.max_body_bytes,
            )
            captured: bytes | None = None
        else:
            captured = body

        # Put the body back on the receive channel so route handlers can read
        # request.body() / request.json() as usual.
        request._receive = _make_replay_receive(body)
        request.state.gov_body = captured
        return captured

    async def _passthrough_body(self, request: Request) -> None:
        """For oversize bodies we still must let the downstream handler read
        the original stream — we just decline to capture it. We don't buffer
        the bytes in-process to keep memory bounded."""
        # No-op: the original receive channel is untouched, so downstream
        # handlers get the raw stream from the ASGI server. Return None to
        # signal "not captured".
        return None

    def _enqueue_sample(
        self,
        request: Request,
        response: Response,
        body: bytes | None,
    ) -> None:
        """Push a sample onto the bounded queue, dropping (with logging) if
        the queue is full. Never blocks the response path."""
        if self.config.sample_rate < 1.0 and random.random() > self.config.sample_rate:
            return
        if self._queue is None:
            # Worker hasn't started — startup hook didn't run. Drop quietly
            # rather than crash; this happens in tests that bypass lifespan.
            return

        sample = {
            "method": request.method,
            "path": request.url.path,
            "status_code": response.status_code,
            "request_body": _decode_json_or_none(body, request.headers.get("content-type", "")),
            "headers": _sanitize_headers(request.headers),
        }
        try:
            self._queue.put_nowait(sample)
        except asyncio.QueueFull:
            self._dropped += 1
            if self._dropped % 100 == 1:
                logger.warning(
                    "api-gov: traffic queue full, dropped %d samples so far",
                    self._dropped,
                )

    async def _ingest_worker(self) -> None:
        """Background coroutine that BATCHES queued samples and POSTs them to
        the backend in a single request.

        Loop shape:
          1. Block on the first sample (so an idle worker doesn't burn CPU).
          2. Drain additional samples up to ``batch_max_size`` OR until
             ``batch_max_wait_ms`` has elapsed since the first sample landed,
             whichever comes first.
          3. POST the whole batch to /api/v1/traffic/batch.
          4. Mark every sample done so :meth:`_drain` (used in shutdown +
             tests) can wait for completion via ``Queue.join()``.

        Failures are logged but never propagate — a transient network blip
        must not permanently disable traffic capture, and we must never
        leave queue items un-task_done'd (which would deadlock shutdown).
        """
        assert self._queue is not None
        while True:
            # Step 1: block until we have something to send.
            try:
                first = await self._queue.get()
            except asyncio.CancelledError:
                raise
            batch: list[dict[str, Any]] = [first]

            # Step 2: greedy drain bounded by size + wall-clock budget.
            deadline = asyncio.get_event_loop().time() + self.config.batch_max_wait_ms / 1000.0
            while len(batch) < self.config.batch_max_size:
                remaining = deadline - asyncio.get_event_loop().time()
                if remaining <= 0:
                    break
                try:
                    sample = await asyncio.wait_for(self._queue.get(), timeout=remaining)
                except asyncio.TimeoutError:
                    break
                except asyncio.CancelledError:
                    # Mark the first sample done before propagating so
                    # shutdown's queue.join() doesn't hang on it.
                    self._queue.task_done()
                    for _ in batch[1:]:
                        self._queue.task_done()
                    raise
                batch.append(sample)

            # Step 3: flush the batch.
            try:
                await self._flush_batch(batch)
            finally:
                # Step 4: always mark every item done so the queue stays
                # consistent even if the POST raised an unexpected error.
                for _ in batch:
                    self._queue.task_done()

    async def _flush_batch(self, batch: list[dict[str, Any]]) -> None:
        """POST a list of samples to /api/v1/traffic/batch. Logs and
        suppresses errors so the worker loop keeps running."""
        if not batch:
            return
        with tracer.start_as_current_span("middleware.ingest_traffic_batch") as span:
            span.set_attribute("batch.size", len(batch))
            try:
                payload = {
                    "spec_id": self.config.spec_id,
                    "samples": [self._sample_to_wire(s) for s in batch],
                }
                resp = await self._client.post(
                    "/api/v1/traffic/batch",
                    json=payload,
                    headers=self._auth_headers(),
                )
                if not resp.is_success:
                    logger.warning(
                        "api-gov traffic batch ingest: HTTP %d for %d samples",
                        resp.status_code,
                        len(batch),
                    )
            except httpx.RequestError as e:
                span.record_exception(e)
                logger.warning(
                    "api-gov traffic batch ingest network error (%d samples): %s",
                    len(batch),
                    e,
                )
            except Exception as e:  # pragma: no cover - defensive
                span.record_exception(e)
                logger.exception(
                    "api-gov traffic batch ingest unexpected error (%d samples): %s",
                    len(batch),
                    e,
                )

    @staticmethod
    def _sample_to_wire(sample: dict[str, Any]) -> dict[str, Any]:
        """Project an in-memory sample onto the on-the-wire shape the
        backend expects. Keeping this as a small, pure helper makes the
        worker loop trivially testable."""
        return {
            "method": sample["method"],
            "path": sample["path"],
            "status_code": sample["status_code"],
            "request": sample["request_body"],
            "response": {"status_code": sample["status_code"]},
            "headers": sample["headers"],
        }

    # ------------------------------------------------------------------
    # Helpers.
    # ------------------------------------------------------------------

    def _auth_headers(self) -> dict[str, str]:
        h = {"Content-Type": "application/json"}
        if self.config.api_key:
            h["Authorization"] = f"Bearer {self.config.api_key}"
        return h

    @property
    def dropped_sample_count(self) -> int:
        """Total samples dropped due to a full ingest queue. Exposed for
        tests and metrics."""
        return self._dropped


# ----------------------------------------------------------------------
# Module-level helpers (pure, easy to unit-test).
# ----------------------------------------------------------------------


def _make_replay_receive(body: bytes) -> _Receive:
    """Return a receive callable that delivers ``body`` exactly once, then
    indicates end-of-stream — matching the ASGI HTTP receive protocol.
    """
    sent = False

    async def receive() -> dict[str, Any]:
        nonlocal sent
        if not sent:
            sent = True
            return {"type": "http.request", "body": body, "more_body": False}
        # After end-of-stream, ASGI servers send disconnect. Mimic that so a
        # handler that keeps polling doesn't loop.
        return {"type": "http.disconnect"}

    return receive


def _decode_json_or_none(body: bytes | None, content_type: str) -> Any:
    """Decode ``body`` if it appears to be JSON. Returns ``None`` otherwise.

    Case-insensitive on the content-type. Tolerant of charset suffixes
    (e.g. ``application/json; charset=utf-8``). Non-JSON bodies are dropped
    rather than blindly converted to strings — the gov DB stores structured
    request samples.
    """
    if body is None or not body:
        return None
    if "application/json" not in content_type.lower():
        return None
    try:
        import json

        return json.loads(body)
    except (ValueError, UnicodeDecodeError):
        return None


def _sanitize_headers(headers: Any) -> dict[str, str]:
    """Return a header dict suitable for storage: lowercase keys, hop-by-hop
    and sensitive headers stripped."""
    out: dict[str, str] = {}
    for k, v in headers.items():
        lk = k.lower()
        if lk in _HOP_BY_HOP_HEADERS:
            continue
        out[lk] = v
    return out


# ----------------------------------------------------------------------
# Public registration helpers.
# ----------------------------------------------------------------------


def setup_lifespan(
    app: FastAPI,
    middleware: APIGovMiddleware,
    *,
    user_lifespan: Callable[[FastAPI], AsyncIterator[None]] | None = None,
) -> None:
    """Wire ``middleware.lifespan`` into the app's lifespan stack.

    If the caller already has a lifespan handler, pass it as ``user_lifespan``
    and we'll compose: api-gov startup → user startup → user shutdown →
    api-gov shutdown.
    """

    @asynccontextmanager
    async def composed(target: FastAPI) -> AsyncIterator[None]:
        async with middleware.lifespan(target):
            if user_lifespan is not None:
                async with user_lifespan(target) as _:
                    yield
            else:
                yield

    # FastAPI stores the lifespan on the router. Setting it here is the
    # public API surface — `app = FastAPI(lifespan=...)` does the same thing
    # before the app starts serving.
    app.router.lifespan_context = composed  # type: ignore[attr-defined]


def register(
    app: FastAPI,
    config: APIGovConfig | None = None,
    *,
    http_client: httpx.AsyncClient | None = None,
) -> APIGovMiddleware:
    """One-call install: add the middleware AND wire lifespan.

    Returns the middleware instance so the caller can inspect counters or
    pass it to tests.
    """
    middleware = APIGovMiddleware(app, config, http_client=http_client)
    app.add_middleware(BaseHTTPMiddleware, dispatch=middleware.dispatch)
    setup_lifespan(app, middleware)
    return middleware
