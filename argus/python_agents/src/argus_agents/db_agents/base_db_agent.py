from __future__ import annotations

import json
from abc import abstractmethod
from typing import Any

import httpx
from pydantic import BaseModel, Field

from argus_agents.base import BaseAgent
from argus_agents.config import AgentConfig
from argus_agents.models import Severity


class DBFinding(BaseModel):
	category: str
	severity: Severity
	title: str
	evidence: list[str]
	action: str


class DBRecommendation(BaseModel):
	summary: str
	health_score: int = Field(ge=0, le=100)
	findings: list[DBFinding]
	proactive_actions: list[str] = Field(default_factory=list)


class BaseDBAgent(BaseAgent):
	# Subclasses override db_type — drives the MCP tool name `db_analyze_<db_type>`.
	db_type: str = "generic"

	def __init__(self, config: AgentConfig | None = None, connection_id: str = "") -> None:
		super().__init__(config)
		self.connection_id = connection_id
		# Separate client targets the Argus backend, not the LLM endpoint exposed by BaseAgent.
		self._backend_client: httpx.Client | None = None

	@property
	def system_prompt(self) -> str:
		return (
			"You are an Argus Database Agent. You receive raw database telemetry "
			"(pg_stat_* views, replication state, index usage, locks, cache stats) "
			"and produce structured DBA recommendations as JSON.\n\n"
			"Rules:\n"
			"1. Cite specific metric values as evidence — never invent numbers.\n"
			"2. Map findings to categories: capacity, performance, reliability, security.\n"
			"3. Assign severity proportional to user impact, not metric magnitude.\n"
			"4. Each finding has a concrete action (a SQL statement, config knob, or operational step).\n"
			"5. health_score reflects overall posture: 90+ healthy, 70-89 minor issues, "
			"50-69 attention required, <50 incident-level."
		)

	# Backend transport is separate from the LLM client so retries/timeouts can diverge.
	@property
	def backend_client(self) -> httpx.Client:
		if self._backend_client is None:
			base = getattr(self.config, "backend_url", None) or "http://127.0.0.1:8765"
			self._backend_client = httpx.Client(base_url=base, timeout=self.config.request_timeout)
		return self._backend_client

	@abstractmethod
	def _fetch_metrics(self, section: str) -> dict[str, Any]:
		"""Return raw metrics for `section` from the Argus backend MCP tool."""

	def analyze(self, section: str) -> dict[str, Any]:
		return self._fetch_metrics(section)

	def recommend(self, section: str = "overview") -> DBRecommendation:
		metrics = self.analyze(section)
		prompt = (
			f"Database type: {self.db_type}\n"
			f"Connection: {self.connection_id}\n"
			f"Section: {section}\n\n"
			f"Raw metrics:\n{json.dumps(metrics, indent=2, default=str)}\n\n"
			"Produce a DBRecommendation. Anchor every finding's evidence in the metrics above."
		)
		result = self.structured_chat(
			[{"role": "user", "content": prompt}],
			DBRecommendation,
		)
		# structured_chat returns BaseModel; cast for typed callers.
		assert isinstance(result, DBRecommendation)
		return result

	def close(self) -> None:
		if self._backend_client is not None:
			self._backend_client.close()
			self._backend_client = None
		super().close()
