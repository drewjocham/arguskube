from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from src.graphs.hacker import load_endpoints, select_target
from src.graphs.state import AgentState


@pytest.mark.asyncio
async def test_select_target_without_endpoint_id() -> None:
    state = AgentState(
        spec_id="spec-1",
        endpoints=[
            {"id": "ep-1", "method": "GET", "path": "/a"},
            {"id": "ep-2", "method": "POST", "path": "/b"},
            {"id": "ep-3", "method": "PUT", "path": "/c"},
            {"id": "ep-4", "method": "DELETE", "path": "/d"},
        ],
        messages=[{"endpoint_id": "", "count": 5}],
    )
    result = await select_target(state)
    assert len(result["endpoints"]) == 3


@pytest.mark.asyncio
async def test_select_target_with_endpoint_id() -> None:
    state = AgentState(
        endpoints=[
            {"id": "ep-1", "method": "GET", "path": "/a"},
            {"id": "ep-2", "method": "POST", "path": "/b"},
        ],
        messages=[{"endpoint_id": "ep-2", "count": 5}],
    )
    result = await select_target(state)
    assert len(result["endpoints"]) == 1
    assert result["endpoints"][0]["id"] == "ep-2"


@pytest.mark.asyncio
async def test_select_target_no_endpoints() -> None:
    state = AgentState(messages=[{"endpoint_id": "", "count": 5}])
    result = await select_target(state)
    assert result["endpoints"] == []


@pytest.mark.asyncio
async def test_generate_returns_test_cases() -> None:
    from src.graphs.hacker import hacker_graph
    with patch.object(hacker_graph, "ainvoke", new_callable=AsyncMock) as mock:
        mock.return_value = {
            "test_cases": [
                {"name": "missing name", "method": "POST", "path": "/users", "expected_status": 400, "description": "test"}
            ]
        }
        from src.graphs.hacker import generate
        result = await generate("spec-1")
        assert len(result) == 1
        assert result[0]["name"] == "missing name"
