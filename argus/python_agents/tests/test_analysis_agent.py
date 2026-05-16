"""Tests for argus_agents.analysis_agent.

The MetricsAgent, TrendAgent, and AnalysisAgent classes are thin
orchestrators over BaseAgent — the value here is locking in the
Pydantic output shape and the EVENT_ANALYSIS_COMPLETE emission. The
shared conftest mocks httpx so no real LLM call goes out.
"""

from __future__ import annotations

import pytest
from pydantic import ValidationError

from argus_agents.analysis_agent import (
    AnalysisAgent,
    AnalysisOutput,
    Degradation,
    MetricsAgent,
    SLOStatus,
    TrendAgent,
)
from argus_agents.event_bus import EVENT_ANALYSIS_COMPLETE


# ─── Pydantic models ──────────────────────────────────────────────────


def test_slo_status_defaults_to_zero() -> None:
    s = SLOStatus(name="latency-p99", status="ok")
    assert s.current_value == 0.0
    assert s.target == 0.0


def test_degradation_defaults_severity_to_medium() -> None:
    d = Degradation(resource="pod/api-1", issue="OOMKilled")
    assert d.severity == "medium"
    assert d.impact == ""


def test_analysis_output_rejects_score_out_of_range() -> None:
    with pytest.raises(ValidationError):
        AnalysisOutput(
            cluster_health_score=150,  # > 100 → must reject
            degradations=[],
            trends=[],
            resource_hotspots=[],
            slo_statuses=[],
        )

    with pytest.raises(ValidationError):
        AnalysisOutput(
            cluster_health_score=-5,  # < 0 → must reject
            degradations=[],
            trends=[],
            resource_hotspots=[],
            slo_statuses=[],
        )


def test_analysis_output_accepts_boundary_values() -> None:
    AnalysisOutput(
        cluster_health_score=0,
        degradations=[],
        trends=[],
        resource_hotspots=[],
        slo_statuses=[],
    )
    AnalysisOutput(
        cluster_health_score=100,
        degradations=[],
        trends=[],
        resource_hotspots=[],
        slo_statuses=[],
    )


# ─── System prompts ───────────────────────────────────────────────────


def test_metrics_agent_system_prompt_is_specific(config, event_bus) -> None:
    agent = MetricsAgent(config)
    prompt = agent.system_prompt
    assert "metrics" in prompt.lower()
    # The prompt must instruct the agent to be specific about numbers —
    # this is the contract that keeps the sub-agent's output useful for
    # the structured AnalysisOutput merge.
    assert "specific" in prompt.lower()


def test_trend_agent_system_prompt_mentions_time_dimension(config, event_bus) -> None:
    agent = TrendAgent(config)
    prompt = agent.system_prompt
    assert "trend" in prompt.lower()


def test_analysis_agent_system_prompt_lists_responsibilities(config, event_bus) -> None:
    agent = AnalysisAgent(config)
    prompt = agent.system_prompt
    # The prompt enumerates the responsibilities; without these the
    # downstream chat can't map back to AnalysisOutput shape.
    assert "health score" in prompt.lower()
    assert "trend" in prompt.lower() or "trends" in prompt.lower()


# ─── End-to-end with mocked HTTP ──────────────────────────────────────


def test_analyze_returns_validated_output_and_emits_event(config, event_bus) -> None:
    received: list[str] = []
    event_bus.on(EVENT_ANALYSIS_COMPLETE, lambda evt: received.append(evt.event_type))

    agent = AnalysisAgent(config)
    agent.bus = event_bus
    out = agent.analyze("cpu=80% mem=4Gi pods=12")

    assert isinstance(out, AnalysisOutput)
    # The fake response in conftest sets cluster_health_score=50.
    assert 0 <= out.cluster_health_score <= 100
    # And the event must have fired exactly once.
    assert received == [EVENT_ANALYSIS_COMPLETE]


def test_analyze_freeform_returns_string(config, event_bus) -> None:
    agent = AnalysisAgent(config)
    result = agent.analyze_freeform("cpu=80% mem=4Gi pods=12")
    assert isinstance(result, str)
    assert len(result) > 0


def test_metrics_agent_analyze_returns_string(config, event_bus) -> None:
    agent = MetricsAgent(config)
    result = agent.analyze("cpu=80% mem=4Gi pods=12")
    assert isinstance(result, str)


def test_trend_agent_analyze_passes_window_through(config, event_bus) -> None:
    # We can't observe the prompt sent (httpx is mocked at client
    # level), but we can confirm the call shape doesn't raise.
    agent = TrendAgent(config)
    result = agent.analyze("cpu=80% mem=4Gi pods=12", "24h")
    assert isinstance(result, str)
