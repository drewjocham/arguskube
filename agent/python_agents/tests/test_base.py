from __future__ import annotations

from typing import Any
from unittest.mock import MagicMock, patch

import pytest

from argus_agents.base import BaseAgent, LLMError, SubAgentPipeline
from argus_agents.models import AgentResult


class MinimalAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return "You are a test agent."


class TestBaseAgent:
    def test_agent_name_is_class_name(self, config: Any) -> None:
        agent = MinimalAgent(config)
        assert agent.agent_name == "MinimalAgent"

    def test_initialises_with_config(self, config: Any) -> None:
        agent = MinimalAgent(config)
        assert agent.config.api_key == "test-key"
        assert agent.config.model == "test-model"

    def test_client_lazy_initialised(self, config: Any) -> None:
        agent = MinimalAgent(config)
        client = agent.client
        assert client is not None
        assert agent._client is client

    def test_build_messages_without_session(self, config: Any) -> None:
        agent = MinimalAgent(config)
        messages = agent.build_messages("hello")
        assert len(messages) == 1
        assert messages[0]["role"] == "user"
        assert messages[0]["content"] == "hello"

    def test_build_messages_with_context(self, config: Any) -> None:
        agent = MinimalAgent(config)
        messages = agent.build_messages("hello", context="some context")
        assert len(messages) == 1
        assert "some context" in messages[0]["content"]

    def test_build_messages_includes_recent_history(self, config: Any, session_context: Any) -> None:
        agent = MinimalAgent(config)
        agent.session = session_context
        from argus_agents.models import ChatMessage
        agent.session.add_message(ChatMessage(role="assistant", content="previous reply"))
        messages = agent.build_messages("new query")
        roles = [m["role"] for m in messages]
        contents = [m["content"] for m in messages]
        assert "previous reply" in contents
        assert "new query" in contents

    def test_execute_calls_chat(self, config: Any) -> None:
        agent = MinimalAgent(config)
        result = agent.execute(prompt="test prompt")
        assert isinstance(result, str)
        assert len(result) > 0

    def test_close_cleans_up_client(self, config: Any) -> None:
        agent = MinimalAgent(config)
        _ = agent.client
        agent.close()
        assert agent._client is None

    def test_emit_without_bus(self, config: Any) -> None:
        agent = MinimalAgent(config)
        event = agent.emit("test.event", {"key": "value"})
        assert event.event_type == "test.event"
        assert event.source_agent == "MinimalAgent"

    def test_emit_with_bus(self, config: Any, event_bus: Any) -> None:
        agent = MinimalAgent(config)
        agent.bus = event_bus
        results: list[str] = []
        event_bus.on("test.event", lambda e: results.append("ok"))
        agent.emit("test.event")
        assert results == ["ok"]


class TestSubAgentPipeline:
    def test_run_success(self, config: Any) -> None:
        parent = MinimalAgent(config)
        pipeline = SubAgentPipeline(parent)
        result = pipeline.run(MinimalAgent)
        assert isinstance(result, AgentResult)
        assert result.success
        assert result.agent_name == "MinimalAgent"

    def test_merge_results(self, config: Any) -> None:
        parent = MinimalAgent(config)
        pipeline = SubAgentPipeline(parent)
        pipeline.run(MinimalAgent, prompt="first")
        pipeline.run(MinimalAgent, prompt="second")
        merged = pipeline.merge_results()
        assert "first" in merged or "second" in merged or merged != ""


class TestLLMError:
    def test_is_exception(self) -> None:
        err = LLMError("something went wrong")
        assert isinstance(err, Exception)
        assert str(err) == "something went wrong"
