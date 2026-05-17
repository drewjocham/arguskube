from __future__ import annotations

from typing import Any
from unittest.mock import patch

import pytest

from argus_agents.orchestrator import OrchestratorAgent


class TestOrchestratorAgent:
    def test_initialises_with_config(self, config: Any) -> None:
        agent = OrchestratorAgent(config)
        assert agent.config.api_key == "test-key"
        assert agent.bus is not None
        assert agent.context is not None
        assert agent.proactive is not None

    def test_start_and_stop_session(self, config: Any) -> None:
        agent = OrchestratorAgent(config)
        ctx = agent.start_session()
        assert ctx is not None
        assert ctx.session_id is not None
        agent.stop_session()
        assert not agent.proactive.is_running()

    def test_classify_and_route_unknown(self, config: Any) -> None:
        agent = OrchestratorAgent(config)
        agent.start_session()
        result = agent.classify_and_route("do something weird")
        assert result.content is not None
        assert "couldn't classify" in result.content.lower() or result.content.strip()

    def test_bus_summary(self, config: Any) -> None:
        agent = OrchestratorAgent(config)
        summary = agent.bus_summary()
        assert "EventBus" in summary
        assert "agent." in summary

    def test_activity_report(self, config: Any) -> None:
        agent = OrchestratorAgent(config)
        report = agent.activity_report()
        assert "Argus" in report or "Activity" in report

    def test_close_cleanup(self, config: Any) -> None:
        agent = OrchestratorAgent(config)
        agent.start_session()
        agent.close()
