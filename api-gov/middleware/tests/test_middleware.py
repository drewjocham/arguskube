from __future__ import annotations

from unittest.mock import AsyncMock, patch

import httpx
import pytest
from fastapi.testclient import TestClient

from src.middleware import APIGovConfig, APIGovMiddleware


def test_middleware_injects_headers(test_app) -> None:
    config = APIGovConfig(api_url="http://localhost:8080", spec_id="spec-1", api_key="test-key")
    APIGovMiddleware(test_app, config)

    with TestClient(test_app) as client:
        resp = client.get("/health")
        assert resp.status_code == 200


@pytest.mark.asyncio
async def test_traffic_ingest_sends_request(test_app) -> None:
    config = APIGovConfig(api_url="http://localhost:8080", spec_id="spec-1")
    mw = APIGovMiddleware(test_app, config)

    with patch.object(mw._client, "post", new_callable=AsyncMock) as mock_post:
        mock_post.return_value = httpx.Response(202, json={"status": "ingested"})
        from starlette.testclient import TestClient as TC

        with TC(test_app) as client:
            resp = client.post("/users", json={"name": "test"})
            assert resp.status_code == 200

        assert mock_post.called
        call_kwargs = mock_post.call_args.kwargs
        assert "spec_id" in call_kwargs["json"]


def test_middleware_retries_on_failure(test_app) -> None:
    config = APIGovConfig(api_url="http://localhost:8080", retry_max=2)
    mw = APIGovMiddleware(test_app, config)

    with patch.object(mw._client, "post", side_effect=httpx.RequestError("fail")) as mock_post:
        import asyncio
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        try:
            loop.run_until_complete(mw._push_spec())
        finally:
            loop.close()

        assert mock_post.call_count == 2


@pytest.mark.asyncio
async def test_sampling_skips_traffic(test_app) -> None:
    config = APIGovConfig(api_url="http://localhost:8080", spec_id="spec-1", sample_rate=0.0)
    mw = APIGovMiddleware(test_app, config)

    with patch.object(mw._client, "post", new_callable=AsyncMock) as mock_post:
        from starlette.testclient import TestClient as TC
        with TC(test_app) as client:
            resp = client.get("/health")
            assert resp.status_code == 200

        assert not mock_post.called
