from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest


@pytest.fixture(autouse=True)
def mock_db() -> None:
    with patch("src.database.db") as mock:
        mock.fetch_endpoints = AsyncMock(return_value=[])
        mock.fetch_spec = AsyncMock(return_value={"id": "test", "content": {}})
        mock.fetch_pending_drifts = AsyncMock(return_value=[])
        mock.save_drift_reports = AsyncMock()
        mock.save_generated_tests = AsyncMock()
        yield


@pytest.fixture(autouse=True)
def mock_redis() -> None:
    with patch("src.redis_client.redis_client") as mock:
        mock.push_traffic = AsyncMock(return_value=1)
        mock.pop_traffic_batch = AsyncMock(return_value=[])
        mock.traffic_count = AsyncMock(return_value=0)
        mock.get_profile = AsyncMock(return_value=None)
        mock.update_profile = AsyncMock()
        yield


@pytest.fixture
def sample_endpoints() -> list[dict]:
    return [
        {
            "id": "ep-1",
            "method": "GET",
            "path": "/users",
            "summary": "List users",
            "tags": ["users"],
            "security": ["api_key"],
            "parameters": [{"name": "page", "in": "query", "schema": {"type": "integer"}}],
        },
        {
            "id": "ep-2",
            "method": "POST",
            "path": "/users",
            "summary": "Create user",
            "tags": ["users"],
            "security": ["api_key"],
            "request_body": {"type": "object", "properties": {"name": {"type": "string"}, "email": {"type": "string"}}},
        },
    ]
