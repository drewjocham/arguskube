from __future__ import annotations

from datetime import datetime

from argus_agents.models import (
    ActionType,
    AgentResult,
    ChatMessage,
    HabitPattern,
    MemoryEntry,
    ProactiveInsight,
    SessionContext,
    Severity,
    TaskClassification,
    UserAction,
)


class TestSessionContext:
    def test_touch_updates_last_active(self) -> None:
        ctx = SessionContext(session_id="s1", user_id="u1")
        before = ctx.last_active
        ctx.touch()
        assert ctx.last_active >= before

    def test_add_action_maintains_limit(self) -> None:
        ctx = SessionContext(session_id="s1", user_id="u1")
        for i in range(150):
            ctx.add_action(UserAction(action_type=ActionType.query, target=f"r{i}"))
        assert len(ctx.recent_actions) <= 100

    def test_add_message_maintains_limit(self) -> None:
        ctx = SessionContext(session_id="s1", user_id="u1")
        for i in range(80):
            ctx.add_message(ChatMessage(role="user", content=f"msg{i}"))
        assert len(ctx.conversation) <= 50

    def test_add_message_touches_timestamp(self) -> None:
        ctx = SessionContext(session_id="s1", user_id="u1")
        before = ctx.last_active
        ctx.add_message(ChatMessage(role="user", content="hello"))
        assert ctx.last_active >= before


class TestUserAction:
    def test_defaults(self) -> None:
        action = UserAction(action_type=ActionType.view, target="pod-1")
        assert action.namespace is None
        assert action.cluster is None
        assert action.duration_ms is None
        assert action.tokens_used is None
        assert isinstance(action.timestamp, datetime)

    def test_with_all_fields(self) -> None:
        action = UserAction(
            action_type=ActionType.diagnose,
            target="deployment/nginx",
            namespace="production",
            cluster="prod-cluster",
            duration_ms=1500,
            tokens_used=42,
            metadata={"severity": "high"},
        )
        assert action.action_type == ActionType.diagnose
        assert action.duration_ms == 1500


class TestChatMessage:
    def test_valid_roles(self) -> None:
        for role in ("system", "user", "assistant", "tool"):
            msg = ChatMessage(role=role, content="test")
            assert msg.role == role

    def test_default_timestamp(self) -> None:
        msg = ChatMessage(role="user", content="hello")
        assert isinstance(msg.timestamp, datetime)


class TestMemoryEntry:
    def test_default_confidence(self) -> None:
        entry = MemoryEntry(id="m1", entry_type="fact", content="something")
        assert entry.confidence == 0.5
        assert entry.user_id == "default"

    def test_expiry_optional(self) -> None:
        entry = MemoryEntry(id="m1", entry_type="fact", content="x")
        assert entry.expiry is None


class TestAgentResult:
    def test_defaults(self) -> None:
        result = AgentResult(agent_name="TestAgent")
        assert result.success
        assert result.content == ""
        assert result.structured is None
        assert result.error is None
        assert result.duration_ms == 0
        assert result.confidence == 0.5

    def test_with_error(self) -> None:
        result = AgentResult(agent_name="TestAgent", success=False, error="something broke")
        assert not result.success
        assert result.error == "something broke"


class TestProactiveInsight:
    def test_default_state(self) -> None:
        ins = ProactiveInsight(
            id="i1",
            trigger_pattern="test",
            confidence=0.8,
            suggestion="pre-fetch data",
            rationale="user habit",
            action_type="notify",
        )
        assert not ins.auto_executed
        assert ins.requires_approval
        assert not ins.acknowledged
        assert ins.result_summary is None


class TestTaskClassification:
    def test_defaults(self) -> None:
        tc = TaskClassification(task_type="chat")
        assert tc.confidence == 0.0
        assert tc.urgency == "medium"
        assert tc.reasoning == ""


class TestHabitPattern:
    def test_minimal(self) -> None:
        hp = HabitPattern(
            id="p1",
            pattern_type="temporal",
            description="user checks pods at 9am",
            predicted_next_action="check pods",
        )
        assert hp.observation_count == 1
        assert hp.confidence == 0.0

    def test_increments_observation(self) -> None:
        hp = HabitPattern(
            id="p1",
            pattern_type="frequency",
            description="frequent viewer",
            observation_count=10,
        )
        assert hp.observation_count == 10


class TestSeverity:
    def test_values(self) -> None:
        assert Severity.critical.value == "critical"
        assert Severity.high.value == "high"
        assert Severity.medium.value == "medium"
        assert Severity.low.value == "low"

    def test_ordering(self) -> None:
        severities = [Severity.low, Severity.info, Severity.high, Severity.critical, Severity.medium]
        assert len(severities) == 5


class TestActionType:
    def test_all_types(self) -> None:
        types = {t.value for t in ActionType}
        expected = {"query", "command", "view", "edit", "diagnose",
                     "remediate", "analyze", "security_scan", "cost_check", "docs"}
        assert types == expected
