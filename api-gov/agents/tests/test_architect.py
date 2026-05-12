from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from src.graphs.architect import analyze, load_spec, extract_metadata
from src.graphs.state import AgentState


@pytest.mark.asyncio
async def test_analyze_returns_analysis_dict() -> None:
    with patch("src.graphs.architect.architect_graph.ainvoke", new_callable=AsyncMock) as mock:
        mock.return_value = {
            "analysis": {"endpoints": 2, "critical_paths": 1, "auth_routes": 2, "summary": "User management API"}
        }
        result = await analyze("spec-1")
        assert result["endpoints"] == 2
        assert result["critical_paths"] == 1
        assert result["auth_routes"] == 2


@pytest.mark.asyncio
async def test_load_spec_fetches_from_db() -> None:
    from src.database import db
    db.fetch_endpoints = AsyncMock(
        return_value=[{"id": "ep-1", "method": "GET", "path": "/test"}]
    )
    db.fetch_spec = AsyncMock(return_value={"id": "spec-1", "content": {"openapi": "3.1.0"}})

    state = AgentState(spec_id="spec-1")
    result = await load_spec(state)
    assert len(result["endpoints"]) == 1
    assert result["spec_content"] == {"openapi": "3.1.0"}


@pytest.mark.asyncio
async def test_extract_metadata_counts_methods() -> None:
    state = AgentState(
        endpoints=[
            {"method": "GET", "path": "/a"},
            {"method": "POST", "path": "/b"},
            {"method": "PUT", "path": "/c"},
            {"method": "DELETE", "path": "/d"},
        ]
    )
    result = await extract_metadata(state)
    assert result["analysis"]["total_endpoints"] == 4
    assert result["analysis"]["critical_count"] == 3


@pytest.mark.asyncio
async def test_extract_metadata_empty_endpoints() -> None:
    state = AgentState()
    result = await extract_metadata(state)
    assert result["analysis"]["total_endpoints"] == 0
    assert result["analysis"]["critical_count"] == 0
    assert result["analysis"]["auth_count"] == 0
