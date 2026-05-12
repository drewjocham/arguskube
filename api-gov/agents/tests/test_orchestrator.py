from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from src.graphs.orchestrator import profile_user, spot_check
from src.graphs.state import AgentState


@pytest.mark.asyncio
async def test_profile_user_increments_session() -> None:
    from src.redis_client import redis_client
    redis_client.update_profile = AsyncMock()

    state = AgentState(spec_id="spec-1", user_profile={"skill_level": "expert", "session_count": 5})
    result = await profile_user(state)
    assert result["user_profile"]["session_count"] == 6


@pytest.mark.asyncio
async def test_profile_user_new_profile() -> None:
    from src.redis_client import redis_client
    redis_client.update_profile = AsyncMock()

    state = AgentState(spec_id="spec-1", user_profile=None)
    result = await profile_user(state)
    assert result["user_profile"]["session_count"] == 1


@pytest.mark.asyncio
async def test_spot_check_passes_by_default() -> None:
    state = AgentState(analysis={"endpoints": 2, "summary": "test"})
    result = await spot_check(state)
    assert result.get("spot_check_passed") is True


@pytest.mark.asyncio
async def test_dispatch_analyze() -> None:
    from src.graphs.orchestrator import orchestrator_graph
    with patch.object(orchestrator_graph, "ainvoke", new_callable=AsyncMock) as mock:
        mock.return_value = {
            "analysis": {"endpoints": 2, "critical_paths": 1, "auth_routes": 0, "summary": "OK"},
            "spot_check_passed": True,
        }
        from src.graphs.orchestrator import orchestrate
        result = await orchestrate("spec-1", action="analyze")
        assert result["analysis"]["endpoints"] == 2
        assert result["spot_check_passed"] is True
