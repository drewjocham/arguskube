from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from src.graphs.sentinel import buffer_traffic, check_threshold, score
from src.graphs.state import AgentState


@pytest.mark.asyncio
async def test_buffer_traffic_stores_event() -> None:
    from src.redis_client import redis_client
    redis_client.push_traffic = AsyncMock(return_value=1)

    state = AgentState(spec_id="spec-1", messages=[{"method": "GET", "path": "/users"}])
    result = await buffer_traffic(state)
    assert len(result["messages"]) == 1


@pytest.mark.asyncio
async def test_check_threshold_below_limit() -> None:
    from src.redis_client import redis_client
    redis_client.traffic_count = AsyncMock(return_value=10)

    state = AgentState(spec_id="spec-1")
    result = await check_threshold(state)
    assert "below threshold" in result["errors"]


@pytest.mark.asyncio
async def test_check_threshold_triggers_scan() -> None:
    from src.redis_client import redis_client
    redis_client.traffic_count = AsyncMock(return_value=50)

    state = AgentState(spec_id="spec-1")
    result = await check_threshold(state)
    assert result["errors"] == []


@pytest.mark.asyncio
async def test_score_filters_by_threshold() -> None:
    state = AgentState(
        drift_reports=[
            {"score": 0.3, "severity": "critical", "category": "type_mismatch"},
            {"score": 0.9, "severity": "low", "category": "missing_field"},
            {"score": 0.5, "severity": "high", "category": "undocumented_field"},
        ]
    )
    result = await score(state)

    scores = [r["score"] for r in result["drift_reports"]]
    assert all(s < 0.85 for s in scores)
    assert len(result["drift_reports"]) == 2


@pytest.mark.asyncio
async def test_score_empty_reports() -> None:
    state = AgentState()
    result = await score(state)
    assert result["drift_reports"] == []
