from __future__ import annotations

from typing import Any
from unittest.mock import MagicMock, patch

import pytest
from pydantic import ValidationError

from argus_agents.db_agents import BaseDBAgent, DBFinding, DBRecommendation, PostgresAgent
from argus_agents.models import Severity


class _StubDBAgent(BaseDBAgent):
	db_type = "stub"

	def _fetch_metrics(self, section: str) -> dict[str, Any]:
		return {"section": section, "value": 42}


class TestDBModels:
	def test_finding_requires_fields(self) -> None:
		f = DBFinding(
			category="performance",
			severity=Severity.high,
			title="Slow query",
			evidence=["mean_exec_time=1500ms"],
			action="CREATE INDEX ...",
		)
		assert f.severity == Severity.high

	def test_recommendation_health_score_bounds(self) -> None:
		with pytest.raises(ValidationError):
			DBRecommendation(summary="x", health_score=150, findings=[])
		with pytest.raises(ValidationError):
			DBRecommendation(summary="x", health_score=-1, findings=[])

	def test_recommendation_defaults_proactive_actions(self) -> None:
		r = DBRecommendation(summary="ok", health_score=80, findings=[])
		assert r.proactive_actions == []


class TestBaseDBAgent:
	def test_system_prompt_non_empty(self, config: Any) -> None:
		agent = _StubDBAgent(config, connection_id="c1")
		assert agent.system_prompt
		assert len(agent.system_prompt) > 50

	def test_connection_id_stored(self, config: Any) -> None:
		agent = _StubDBAgent(config, connection_id="conn-123")
		assert agent.connection_id == "conn-123"

	def test_analyze_delegates_to_fetch_metrics(self, config: Any) -> None:
		agent = _StubDBAgent(config, connection_id="c1")
		metrics = agent.analyze("overview")
		assert metrics == {"section": "overview", "value": 42}

	def test_recommend_runs_structured_chat(self, config: Any) -> None:
		agent = _StubDBAgent(config, connection_id="c1")
		fake = DBRecommendation(
			summary="healthy",
			health_score=92,
			findings=[
				DBFinding(
					category="performance",
					severity=Severity.low,
					title="One unused index",
					evidence=["idx_scan=0 on idx_foo_bar"],
					action="DROP INDEX idx_foo_bar;",
				),
			],
			proactive_actions=["schedule weekly index review"],
		)
		with patch.object(agent, "structured_chat", return_value=fake) as sc:
			result = agent.recommend(section="overview")
		assert sc.called
		assert isinstance(result, DBRecommendation)
		assert result.health_score == 92
		assert result.findings[0].action.startswith("DROP INDEX")


class TestPostgresAgent:
	def test_db_type_is_postgres(self, config: Any) -> None:
		agent = PostgresAgent(config, connection_id="pg1")
		assert agent.db_type == "postgres"

	def test_system_prompt_mentions_pg_views(self, config: Any) -> None:
		agent = PostgresAgent(config, connection_id="pg1")
		prompt = agent.system_prompt
		assert "pg_stat_activity" in prompt
		assert "pg_stat_statements" in prompt
		assert "replication" in prompt.lower()
		assert "cache hit" in prompt.lower() or "shared_buffers" in prompt

	def test_fetch_metrics_calls_backend(self, config: Any) -> None:
		agent = PostgresAgent(config, connection_id="pg1")
		mock_client = MagicMock()
		mock_resp = MagicMock()
		mock_resp.json.return_value = {"active": 3, "idle_in_tx": 0}
		mock_client.post.return_value = mock_resp
		agent._backend_client = mock_client
		out = agent._fetch_metrics("activity")
		mock_client.post.assert_called_once()
		args, kwargs = mock_client.post.call_args
		assert args[0] == "/api/AnalyzeDB"
		assert kwargs["json"]["connection_id"] == "pg1"
		assert kwargs["json"]["section"] == "activity"
		assert out == {"active": 3, "idle_in_tx": 0}
