"""FastAPI middleware that pushes OpenAPI specs and traffic to API Gov."""

from __future__ import annotations

import logging
import os
from contextlib import asynccontextmanager
from typing import Any

import httpx
from fastapi import FastAPI, Request, Response
from opentelemetry import trace
from pydantic import BaseModel

logger = logging.getLogger(__name__)
tracer = trace.get_tracer("api-gov-middleware")


class APIGovConfig(BaseModel):
    api_url: str = ""
    spec_id: str = ""
    api_key: str = ""
    auto_ingest_traffic: bool = True
    sample_rate: float = 1.0  # 0.0 to 1.0
    retry_max: int = 3


class APIGovMiddleware:
    def __init__(self, app: FastAPI, config: APIGovConfig | None = None) -> None:
        self.app = app
        self.config = config or APIGovConfig(
            api_url=os.getenv("API_GOV_URL", "http://localhost:8080"),
            spec_id=os.getenv("API_GOV_SPEC_ID", ""),
            api_key=os.getenv("API_GOV_API_KEY", ""),
            sample_rate=float(os.getenv("API_GOV_SAMPLE_RATE", "1.0")),
        )
        self._client = httpx.AsyncClient(base_url=self.config.api_url, timeout=10)
        self._register_lifespan()

    def _register_lifespan(self) -> None:
        original_lifespan = getattr(self.app.router, "lifespan", None)

        @asynccontextmanager
        async def wrapped_lifespan(app: FastAPI):
            await self._push_spec()
            async with original_lifespan(app) if original_lifespan else contextlib.nullcontext():
                yield
            await self._client.aclose()

        self.app.router.lifespan = wrapped_lifespan

    async def _push_spec(self) -> None:
        for attempt in range(self.config.retry_max):
            try:
                openapi = self.app.openapi()
                if not openapi:
                    logger.warning("no OpenAPI spec found")
                    return

                name = openapi.get("info", {}).get("title", "unknown")
                version = openapi.get("info", {}).get("version", "1.0.0")

                resp = await self._client.post(
                    "/api/v1/specs",
                    json={"name": name, "version": version, "content": openapi},
                    headers=self._auth_headers(),
                )
                if resp.is_success:
                    data = resp.json()
                    self.config.spec_id = data.get("id", "")
                    logger.info("spec pushed: %s (%s)", name, self.config.spec_id)
                    return
                logger.warning("push attempt %d failed: %s", attempt + 1, resp.status_code)
            except httpx.RequestError as e:
                logger.warning("push attempt %d error: %s", attempt + 1, e)
                if attempt == self.config.retry_max - 1:
                    logger.error("spec push failed after %d retries", self.config.retry_max)

    async def _ingest_traffic(self, request: Request, response: Response) -> None:
        if not self.config.spec_id or not self.config.auto_ingest_traffic:
            return
        if self.config.sample_rate < 1.0 and hash(request.url.path) % 100 > self.config.sample_rate * 100:
            return

        with tracer.start_as_current_span("middleware.ingest_traffic") as span:
            span.set_attribute("method", request.method)
            span.set_attribute("path", request.url.path)
            span.set_attribute("status", response.status_code)

            try:
                body = None
                if request.method in ("POST", "PUT", "PATCH"):
                    ct = request.headers.get("content-type", "")
                    if "application/json" in ct:
                        body = await request.json()

                await self._client.post(
                    "/api/v1/traffic",
                    json={
                        "spec_id": self.config.spec_id,
                        "method": request.method,
                        "path": request.url.path,
                        "status_code": response.status_code,
                        "request": body,
                        "response": {"status_code": response.status_code},
                        "headers": dict(request.headers),
                    },
                    headers=self._auth_headers(),
                )
            except Exception as e:
                span.record_exception(e)
                logger.debug("traffic ingest skipped: %s", e)

    def _auth_headers(self) -> dict[str, str]:
        h = {"Content-Type": "application/json"}
        if self.config.api_key:
            h["Authorization"] = f"Bearer {self.config.api_key}"
        return h

    async def __call__(self, request: Request, call_next: Any) -> Response:
        response = await call_next(request)
        await self._ingest_traffic(request, response)
        return response


def register(app: FastAPI, config: APIGovConfig | None = None) -> APIGovMiddleware:
    middleware = APIGovMiddleware(app, config)
    app.middleware("http")(middleware)
    return middleware
