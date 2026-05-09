from __future__ import annotations

from pydantic import BaseModel, Field

from .base import BaseAgent
from .event_bus import EVENT_ANALYSIS_COMPLETE


class SLOStatus(BaseModel):
    name: str
    status: str  # met, breaching, at_risk
    current_value: float = 0.0
    target: float = 0.0


class Degradation(BaseModel):
    resource: str
    issue: str
    impact: str = ""
    severity: str = "medium"


class AnalysisOutput(BaseModel):
    cluster_health_score: int = Field(ge=0, le=100)
    degradations: list[Degradation]
    trends: list[str]
    resource_hotspots: list[str]
    slo_statuses: list[SLOStatus]
    summary: str = ""


class MetricsAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are a metrics analysis sub-agent. Given CPU, memory, and pod metrics,
identify anomalies, trends, and resource pressure points. Be specific with numbers."""

    def analyze(self, metrics: str) -> str:
        return self.chat([{"role": "user", "content": f"Analyze these metrics:\n{metrics}"}])


class TrendAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are a trend detection sub-agent. Given a time series of metrics,
identify significant trends: increasing error rates, memory creep, restart
frequency changes, and seasonal patterns."""

    def analyze(self, metrics: str, window: str) -> str:
        return self.chat([{"role": "user", "content": f"Detect trends over {window}:\n{metrics}"}])


class AnalysisAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are the Argus Analysis Agent — a cluster performance and health analyst.

Your sole responsibility is **analyzing cluster state, metrics, and trends** to surface insights.

Rules:
1. Analyse pod health, node pressure, resource utilisation.
2. Detect trends: restart counts, memory creep, OOMKilled frequency.
3. Compare against SLO benchmarks.
4. Flag resource imbalance: over-provisioned nodes, under-sized requests.
5. Provide a health score (0–100) with rationale.
"""

    def analyze(self, metrics_snapshot: str, time_window: str = "1h") -> AnalysisOutput:
        pipeline = self.sub_agents()

        pipeline.run(MetricsAgent, prompt=f"Analyze these metrics:\n{metrics_snapshot}")
        pipeline.run(TrendAgent, prompt=f"Detect trends over {time_window}:\n{metrics_snapshot}")

        enriched = f"Metrics ({time_window} window):\n{metrics_snapshot}"
        sub_results = pipeline.merge_results()
        if sub_results:
            enriched = f"{enriched}\n\nSub-agent analysis:\n{sub_results}"

        result = self.structured_chat(
            [{"role": "user", "content": f"Analyze this cluster state:\n{enriched}"}],
            AnalysisOutput,
        )
        self.emit(EVENT_ANALYSIS_COMPLETE, {
            "health_score": result.cluster_health_score,
            "degradation_count": len(result.degradations),
            "hotspots": result.resource_hotspots,
        })
        return result

    def analyze_freeform(self, metrics_snapshot: str, time_window: str = "1h") -> str:
        return self.chat([
            {"role": "user", "content": f"Analyze this cluster state over the last {time_window}:\n{metrics_snapshot}"}
        ])
