from __future__ import annotations

import time
from typing import Any
from unittest.mock import MagicMock

from argus_agents.models import ActionType, ProactiveInsight, UserAction
from argus_agents.proactive_agent import ProactiveAgent


class TestProactiveAgent:
    def test_initialise(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        assert not agent.is_running()
        assert agent.pending_insights() == []
        agent.close()

    def test_observe_adds_action(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        action = UserAction(action_type=ActionType.view, target="pods", namespace="default")
        agent.observe(action)
        assert len(agent._action_buffer) == 1
        agent.close()

    def test_observe_maintains_buffer_limit(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        for i in range(300):
            agent.observe(UserAction(action_type=ActionType.view, target=f"r{i}"))
        assert len(agent._action_buffer) <= 200
        agent.close()

    def test_pending_insights_clear(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        agent._insights.append(ProactiveInsight(
            id="i1", trigger_pattern="test", confidence=0.5,
            suggestion="test", rationale="test", action_type="notify",
        ))
        assert len(agent.pending_insights()) == 1
        assert len(agent.pending_insights()) == 0

    def test_latest_insight(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        assert agent.latest_insight() is None
        agent._insights.append(ProactiveInsight(
            id="i1", trigger_pattern="test", confidence=0.5,
            suggestion="test", rationale="test", action_type="notify",
        ))
        assert agent.latest_insight() is not None

    def test_report_activity_empty(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        assert agent.report_activity() == ""

    def test_report_activity_with_actions(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        agent.observe(UserAction(action_type=ActionType.view, target="pods"))
        report = agent.report_activity()
        assert "view" in report

    def test_temporal_pattern_detection(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        actions = [
            UserAction(action_type=ActionType.view, target="pods/nginx")
            for _ in range(5)
        ]
        patterns = agent._detect_temporal_patterns(actions)
        assert isinstance(patterns, list)

    def test_sequential_pattern_detection(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        actions = [
            UserAction(action_type=ActionType.view, target="pods/nginx"),
            UserAction(action_type=ActionType.view, target="services/nginx"),
            UserAction(action_type=ActionType.view, target="pods/nginx"),
            UserAction(action_type=ActionType.view, target="services/nginx"),
        ]
        patterns = agent._detect_sequential_patterns(actions)
        assert isinstance(patterns, list)

    def test_frequency_pattern_detection(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        actions = [
            UserAction(action_type=ActionType.view, target="pods/nginx")
            for _ in range(6)
        ]
        patterns = agent._detect_frequency_patterns(actions)
        assert isinstance(patterns, list)

    def test_start_stop_lifecycle(self, config: Any) -> None:
        agent = ProactiveAgent(config)
        agent.start()
        assert agent.is_running()
        agent.stop()
        assert not agent.is_running()
        agent.close()
