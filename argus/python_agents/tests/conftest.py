from __future__ import annotations

from typing import Any
from unittest.mock import patch

import pytest

from argus_agents.config import AgentConfig
from argus_agents.event_bus import EventBus, AgentEvent
from argus_agents.models import ChatMessage, SessionContext, UserAction, ActionType


@pytest.fixture
def config(tmp_path: Any) -> AgentConfig:
    return AgentConfig(
        api_key="test-key",
        base_url="http://test.local/v1",
        model="test-model",
        temperature=0.0,
        max_tokens=100,
        request_timeout=5,
        max_retries=0,
        chroma_path=str(tmp_path / "chroma"),
        proactive_interval_seconds=9999,
    )


@pytest.fixture
def event_bus() -> EventBus:
    return EventBus()


@pytest.fixture
def session_context() -> SessionContext:
    return SessionContext(
        session_id="test-session",
        user_id="test-user",
    )


@pytest.fixture
def user_action() -> UserAction:
    return UserAction(
        action_type=ActionType.query,
        target="test-resource",
        namespace="default",
    )


@pytest.fixture
def chat_message() -> ChatMessage:
    return ChatMessage(role="user", content="test message")


@pytest.fixture
def agent_event() -> AgentEvent:
    return AgentEvent(
        event_type="test.event",
        source_agent="TestAgent",
    )


@pytest.fixture(autouse=True)
def mock_httpx_client() -> Any:
    """Prevent any real HTTP calls during tests."""
    with patch("argus_agents.base.httpx.Client") as mock:
        client_instance = mock.return_value
        client_instance.post.return_value.status_code = 200
        client_instance.post.return_value.json.return_value = {
            "choices": [{"message": {"content": json_compatible_fake_response()}}]
        }
        yield mock


def json_compatible_fake_response() -> str:
    """Return a JSON string that passes model_validate_json for all agent models.

    Each model ignores unknown fields, so a superset of all possible fields works.
    """
    return (
        '{"task_type":"chat","confidence":0.5,"reasoning":"test",'
        '"requires_sub_agents":[],"urgency":"medium",'
        '"likely_root_cause":"test","blast_radius":[],"evidence":[],'
        '"next_data_needed":[],"severity":"medium",'
        '"diagnosis_summary":"test","immediate_action":"test",'
        '"steps":[],"verification_commands":[],'
        '"cluster_health_score":50,"degradations":[],"trends":[],'
        '"resource_hotspots":[],"slo_statuses":[],'
        '"risk_score":0,"critical_findings":[],"patch_priority":[],'
        '"compliance_gaps":[],'
        '"monthly_burn_rate":0,"top_savings":[],"rightsizing":[],'
        '"spot_eligible_workloads":[],'
        '"title":"test","alert_type":"test","preflight_checks":[],'
        '"sections":[],"rollback_steps":[],'
        '"summary":"test","timeline":[],"root_cause":"test","impact":"test",'
        '"action_items":[],"lessons_learned":[],'
        '"content":"test response","agent_name":"TestAgent","success":true}'
    )



