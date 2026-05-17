from __future__ import annotations

from typing import Any

from argus_agents.context_agent import ContextAgent
from argus_agents.models import ActionType, UserAction


class TestContextAgent:
    def test_create_session(self, config: Any) -> None:
        agent = ContextAgent(config)
        ctx = agent.create_session("test-user")
        assert ctx.user_id == "test-user"
        assert ctx.session_id is not None

    def test_session_auto_created(self, config: Any) -> None:
        agent = ContextAgent(config)
        session = agent.session
        assert session.user_id == config.user_id

    def test_track_action(self, config: Any) -> None:
        agent = ContextAgent(config)
        action = UserAction(action_type=ActionType.view, target="pods", namespace="default")
        agent.track_action(action)
        assert len(agent.session.recent_actions) == 1
        assert agent.session.recent_actions[0].target == "pods"

    def test_track_message(self, config: Any) -> None:
        agent = ContextAgent(config)
        agent.track_message("user", "hello world")
        assert len(agent.session.conversation) == 1
        assert agent.session.conversation[0].role == "user"
        assert agent.session.conversation[0].content == "hello world"

    def test_recall_returns_empty_string_without_data(self, config: Any) -> None:
        agent = ContextAgent(config)
        result = agent.recall("something random")
        assert result == ""

    def test_session_summary(self, config: Any) -> None:
        agent = ContextAgent(config)
        _ = agent.create_session()
        summary = agent.session_summary()
        assert "Session:" in summary
        assert "Messages:" in summary
        assert "Actions:" in summary

    def test_touch_session(self, config: Any) -> None:
        agent = ContextAgent(config)
        before = agent.session.last_active
        agent.touch_session()
        assert agent.session.last_active >= before

    def test_close(self, config: Any) -> None:
        agent = ContextAgent(config)
        agent.close()
