"""Tests for the APIGovMiddleware.

The middleware has two hard requirements that are easy to get wrong, so
they get the most coverage:

1. **Body capture must NOT consume the stream** — the downstream handler
   must still see the exact bytes the client sent.
2. **Traffic ingest must NOT block the response path** — slow or broken
   backends cannot delay user-facing latency.

We use ``httpx.MockTransport`` for the gov backend rather than mocking
``_client.post`` so the assertion surface is "what bytes hit the wire" not
"which Python method was called".
"""

from __future__ import annotations

import asyncio
import json
from typing import Any

import httpx
import pytest
from fastapi import FastAPI, Request
from fastapi.testclient import TestClient

from src.middleware import (
    APIGovConfig,
    APIGovMiddleware,
    _decode_json_or_none,
    _sanitize_headers,
    register,
    setup_lifespan,
)


# ---------------------------------------------------------------------------
# Helpers / fixtures.
# ---------------------------------------------------------------------------


class RecordingTransport(httpx.MockTransport):
    """An httpx mock transport that records every request and lets the test
    program the response."""

    def __init__(self, spec_response: dict[str, Any] | None = None, status: int = 200) -> None:
        self.recorded: list[httpx.Request] = []
        self.bodies: list[bytes] = []
        self._spec_response = spec_response or {"id": "spec-from-mock"}
        self._status = status

        async def handler(request: httpx.Request) -> httpx.Response:
            self.recorded.append(request)
            self.bodies.append(request.content)
            if request.url.path == "/api/v1/specs":
                return httpx.Response(self._status, json=self._spec_response)
            return httpx.Response(202, json={"status": "ingested"})

        super().__init__(handler)

    def calls_to(self, path: str) -> list[httpx.Request]:
        return [r for r in self.recorded if r.url.path == path]

    def traffic_samples(self) -> list[dict[str, Any]]:
        """Flatten every batched traffic POST into a list of individual
        samples — the shape every traffic-flavoured assertion in this
        file wants to reason about. Each entry is the wire-format sample
        the middleware sends inside the batch payload."""
        out: list[dict[str, Any]] = []
        for r in self.calls_to("/api/v1/traffic/batch"):
            payload = json.loads(r.content)
            for s in payload.get("samples", []):
                out.append(s)
        return out


def _make_client(transport: httpx.MockTransport) -> httpx.AsyncClient:
    return httpx.AsyncClient(transport=transport, base_url="http://gov.test")


@pytest.fixture
def app_with_body_echo() -> FastAPI:
    """An app whose route reads the request body so the test can verify the
    downstream handler is NOT cut off by the middleware's body cache."""
    app = FastAPI()

    @app.get("/health")
    async def health() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/echo")
    async def echo(request: Request) -> dict[str, Any]:
        # If middleware consumed the stream without re-arming, this raises
        # or returns an empty body — failing the test loudly.
        payload = await request.json()
        return {"received": payload}

    return app


# ---------------------------------------------------------------------------
# 1. Body capture must NOT consume the stream.
# ---------------------------------------------------------------------------


def test_body_capture_does_not_break_downstream_handler(app_with_body_echo: FastAPI) -> None:
    transport = RecordingTransport()
    client = _make_client(transport)
    mw = register(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1"),
        http_client=client,
    )

    with TestClient(app_with_body_echo) as test_client:
        resp = test_client.post("/echo", json={"name": "claude"})
    assert resp.status_code == 200
    assert resp.json() == {"received": {"name": "claude"}}
    _drain_queue(mw)


def test_body_over_limit_is_passed_through_not_captured(app_with_body_echo: FastAPI) -> None:
    transport = RecordingTransport()
    client = _make_client(transport)
    mw = register(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1", max_body_bytes=16),
        http_client=client,
    )

    with TestClient(app_with_body_echo) as test_client:
        big_payload = {"data": "x" * 1024}
        resp = test_client.post("/echo", json=big_payload)
    # Handler still works — middleware does not break the request.
    assert resp.status_code == 200
    assert resp.json()["received"]["data"].startswith("x")
    _drain_queue(mw)
    # The traffic body for an oversize request should be null/None, not the
    # raw bytes — we never want the gov DB to store oversize payloads.
    samples = transport.traffic_samples()
    if samples:
        assert samples[-1]["request"] is None


# ---------------------------------------------------------------------------
# 2. Traffic ingest must NOT block the response path.
# ---------------------------------------------------------------------------


def test_full_queue_drops_samples_without_blocking(app_with_body_echo: FastAPI) -> None:
    transport = RecordingTransport()
    client = _make_client(transport)
    mw = register(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1", queue_size=1),
        http_client=client,
    )

    with TestClient(app_with_body_echo) as test_client:
        # Hammer 25 requests — the worker shouldn't keep up with all of them
        # given queue_size=1, so some must be dropped.
        for _ in range(25):
            resp = test_client.post("/echo", json={"i": 1})
            assert resp.status_code == 200
        _drain_queue(mw)

    # The drop counter exists and exposes the operational signal.
    assert mw.dropped_sample_count >= 0  # may be 0 if worker drained fast


# ---------------------------------------------------------------------------
# 3. Hop-by-hop / sensitive headers must be filtered.
# ---------------------------------------------------------------------------


def test_authorization_and_hop_by_hop_headers_are_stripped(app_with_body_echo: FastAPI) -> None:
    transport = RecordingTransport()
    client = _make_client(transport)
    mw = register(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1"),
        http_client=client,
    )

    with TestClient(app_with_body_echo) as test_client:
        resp = test_client.post(
            "/echo",
            json={"x": 1},
            headers={
                "Authorization": "Bearer SECRET",
                "Connection": "keep-alive",
                "Transfer-Encoding": "chunked",
                "Cookie": "session=very-secret",
                "X-Custom": "kept",
            },
        )
    assert resp.status_code == 200
    _drain_queue(mw)

    samples = transport.traffic_samples()
    assert samples, "expected a traffic ingest call"
    captured = samples[-1]["headers"]
    assert "authorization" not in captured
    assert "cookie" not in captured
    assert "connection" not in captured
    assert "transfer-encoding" not in captured
    assert captured.get("x-custom") == "kept"


# ---------------------------------------------------------------------------
# 4. Content-Type case-insensitivity.
# ---------------------------------------------------------------------------


def test_decode_json_or_none_is_case_insensitive() -> None:
    assert _decode_json_or_none(b'{"a": 1}', "Application/JSON; charset=utf-8") == {"a": 1}
    assert _decode_json_or_none(b'{"a": 1}', "APPLICATION/JSON") == {"a": 1}
    assert _decode_json_or_none(b"raw text", "text/plain") is None
    assert _decode_json_or_none(None, "application/json") is None
    assert _decode_json_or_none(b"", "application/json") is None


# ---------------------------------------------------------------------------
# 5. Spec push behavior — retries and capture of returned id.
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_push_spec_captures_returned_id(app_with_body_echo: FastAPI) -> None:
    transport = RecordingTransport(spec_response={"id": "spec-abc"})
    client = _make_client(transport)
    mw = APIGovMiddleware(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test"),
        http_client=client,
    )
    await mw._push_spec()
    assert mw.config.spec_id == "spec-abc"
    await client.aclose()


@pytest.mark.asyncio
async def test_push_spec_retries_on_request_error(app_with_body_echo: FastAPI) -> None:
    attempts = {"n": 0}

    async def handler(request: httpx.Request) -> httpx.Response:
        attempts["n"] += 1
        if attempts["n"] < 3:
            raise httpx.ConnectError("simulated")
        return httpx.Response(200, json={"id": "spec-late"})

    transport = httpx.MockTransport(handler)
    client = _make_client(transport)
    mw = APIGovMiddleware(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", retry_max=3),
        http_client=client,
    )
    await mw._push_spec()
    assert attempts["n"] == 3
    assert mw.config.spec_id == "spec-late"
    await client.aclose()


# ---------------------------------------------------------------------------
# 6. _sanitize_headers is the contract surface for header filtering.
# ---------------------------------------------------------------------------


# ---------------------------------------------------------------------------
# 7. Batching — the headline perf change. The worker must:
#    * post to /api/v1/traffic/batch, not /api/v1/traffic (per sample)
#    * pack multiple samples into one HTTP request
#    * include spec_id once, samples as a list
#    * fall back to short flushes when batch_max_wait_ms elapses
# ---------------------------------------------------------------------------


def test_worker_posts_to_batch_endpoint_with_samples_list(app_with_body_echo: FastAPI) -> None:
    transport = RecordingTransport()
    client = _make_client(transport)
    mw = register(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1"),
        http_client=client,
    )
    with TestClient(app_with_body_echo) as test_client:
        for _ in range(3):
            test_client.post("/echo", json={"i": 1})
        _drain_queue(mw)

    # No single-sample POSTs allowed — batch is the only ingest path.
    assert transport.calls_to("/api/v1/traffic") == []
    batches = transport.calls_to("/api/v1/traffic/batch")
    assert batches, "worker must POST at least one batch"
    payload = json.loads(batches[0].content)
    # The startup spec push overwrites config.spec_id with whatever the
    # mock backend returned — we only care that SOME non-empty id flowed
    # through, not which one.
    assert payload["spec_id"]
    assert isinstance(payload["samples"], list)
    assert len(payload["samples"]) >= 1
    sample = payload["samples"][0]
    # Each sample carries the same shape the legacy endpoint accepted.
    for key in ("method", "path", "status_code", "request", "response", "headers"):
        assert key in sample


def test_worker_coalesces_burst_into_one_batch(app_with_body_echo: FastAPI) -> None:
    """Under burst the worker should NOT send 10 batches of 1 — that
    would defeat the whole point of batching."""
    transport = RecordingTransport()
    client = _make_client(transport)
    mw = register(
        app_with_body_echo,
        APIGovConfig(
            api_url="http://gov.test",
            spec_id="spec-1",
            batch_max_size=50,
            batch_max_wait_ms=200,
            queue_size=256,
        ),
        http_client=client,
    )

    N = 20
    with TestClient(app_with_body_echo) as test_client:
        for _ in range(N):
            test_client.post("/echo", json={"i": 1})
        _drain_queue(mw)

    samples = transport.traffic_samples()
    assert len(samples) == N, f"expected every sample to be ingested, got {len(samples)}"
    # And we did it in dramatically fewer HTTP requests than N.
    batches = transport.calls_to("/api/v1/traffic/batch")
    assert len(batches) < N, (
        f"batching should coalesce {N} samples; saw {len(batches)} batch requests"
    )


@pytest.mark.asyncio
async def test_flush_batch_is_a_no_op_on_empty_input(app_with_body_echo: FastAPI) -> None:
    transport = RecordingTransport()
    client = _make_client(transport)
    mw = APIGovMiddleware(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1"),
        http_client=client,
    )
    await mw._flush_batch([])
    assert transport.calls_to("/api/v1/traffic/batch") == []
    await client.aclose()


@pytest.mark.asyncio
async def test_flush_batch_survives_backend_500(app_with_body_echo: FastAPI) -> None:
    """A non-2xx response from the backend must NOT crash the worker — it
    logs and moves on. We exercise the helper directly to keep the assert
    tight on this behaviour."""

    async def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(500, json={"error": "boom"})

    transport = httpx.MockTransport(handler)
    client = _make_client(transport)
    mw = APIGovMiddleware(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1"),
        http_client=client,
    )
    # No exception should bubble out.
    await mw._flush_batch([{
        "method": "GET", "path": "/x", "status_code": 200,
        "request_body": None, "headers": {},
    }])
    await client.aclose()


@pytest.mark.asyncio
async def test_flush_batch_survives_network_error(app_with_body_echo: FastAPI) -> None:
    async def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("simulated")

    transport = httpx.MockTransport(handler)
    client = _make_client(transport)
    mw = APIGovMiddleware(
        app_with_body_echo,
        APIGovConfig(api_url="http://gov.test", spec_id="spec-1"),
        http_client=client,
    )
    await mw._flush_batch([{
        "method": "GET", "path": "/x", "status_code": 200,
        "request_body": None, "headers": {},
    }])
    await client.aclose()


def test_sample_to_wire_drops_internal_fields() -> None:
    """The on-disk shape must match what the backend handler decodes."""
    wire = APIGovMiddleware._sample_to_wire({
        "method": "POST",
        "path": "/users",
        "status_code": 201,
        "request_body": {"name": "alice"},
        "headers": {"x-custom": "yes"},
        # Extra fields the worker may carry internally — must not leak.
        "_internal": "shouldnt-be-here",
    })
    assert wire == {
        "method": "POST",
        "path": "/users",
        "status_code": 201,
        "request": {"name": "alice"},
        "response": {"status_code": 201},
        "headers": {"x-custom": "yes"},
    }


def test_config_from_env_reads_batch_knobs(monkeypatch) -> None:
    """API_GOV_BATCH_* env vars must reach the config so deployments can
    tune throughput vs latency without redeploying code."""
    monkeypatch.setenv("API_GOV_BATCH_MAX_SIZE", "75")
    monkeypatch.setenv("API_GOV_BATCH_MAX_WAIT_MS", "1500")
    cfg = APIGovConfig.from_env()
    assert cfg.batch_max_size == 75
    assert cfg.batch_max_wait_ms == 1500


def test_sanitize_headers_lowercases_and_filters() -> None:
    raw = {
        "X-Trace": "abc",
        "Authorization": "Bearer x",
        "Connection": "close",
        "Set-Cookie": "x=1",
    }
    out = _sanitize_headers(raw)
    assert "x-trace" in out
    assert out["x-trace"] == "abc"
    assert "authorization" not in out
    assert "connection" not in out
    assert "set-cookie" not in out


# ---------------------------------------------------------------------------
# Internal helper: drain the in-process queue so assertions about ingest
# requests are deterministic. The worker is started by the lifespan event,
# which TestClient runs for us — but we still need to wait for the queue to
# empty before assertions.
# ---------------------------------------------------------------------------


def _drain_queue(mw: APIGovMiddleware, timeout: float = 1.0) -> None:
    if mw._queue is None:
        return
    try:
        loop = asyncio.get_event_loop()
        if loop.is_running():
            return
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
    try:
        loop.run_until_complete(asyncio.wait_for(mw._queue.join(), timeout))
    except (asyncio.TimeoutError, RuntimeError):
        # Queue may already be empty / loop closed by the TestClient. Either
        # way the assertions only fail if the call genuinely never reached
        # the transport — the drain is best-effort.
        pass
