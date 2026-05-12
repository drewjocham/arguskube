from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from src.graphs.healer import calculate_confidence
from src.graphs.state import AgentState


@pytest.mark.asyncio
async def test_calculate_confidence_high_auto_apply() -> None:
    state = AgentState(analysis={"fix": {"confidence": 0.9, "drift_type": "intentional"}})
    result = await calculate_confidence(state)
    assert result["analysis"]["fix"]["auto_apply"] is True


@pytest.mark.asyncio
async def test_calculate_confidence_low_no_auto() -> None:
    state = AgentState(analysis={"fix": {"confidence": 0.5, "drift_type": "bug"}})
    result = await calculate_confidence(state)
    assert result["analysis"]["fix"]["auto_apply"] is False


@pytest.mark.asyncio
async def test_calculate_confidence_no_fix() -> None:
    state = AgentState()
    result = await calculate_confidence(state)
    assert result["analysis"]["fix"]["auto_apply"] is False
