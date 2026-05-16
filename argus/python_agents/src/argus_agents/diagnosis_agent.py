from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field

from argus_agents.base import BaseAgent
from argus_agents.event_bus import EVENT_DIAGNOSIS_COMPLETE
from argus_agents.models import Severity


class DiagnosisOutput(BaseModel):
    likely_root_cause: str
    confidence: float = Field(ge=0.0, le=1.0)
    blast_radius: list[str]
    evidence: list[str]
    next_data_needed: list[str]
    severity: Severity = Severity.medium
    related_patterns: list[str] = Field(default_factory=list)


class LogAnalysisAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are a log analysis sub-agent. Your job is to extract error patterns
and stack traces from pod logs. Be concise. List timestamps and error signatures only."""

    def analyze(self, logs: str) -> str:
        return self.chat([{"role": "user", "content": f"Analyze these logs for errors:\n{logs}"}])


class K8sStateAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are a Kubernetes state analysis sub-agent. Your job is to identify
state anomalies: CrashLoopBackOff, OOMKilled, ImagePullBackOff, Pending pods,
NodePressure, and other scheduler issues."""

    def analyze(self, state: str) -> str:
        return self.chat([{"role": "user", "content": f"Analyze this cluster state for anomalies:\n{state}"}])


class DiagnosisAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are the Argus Diagnosis Agent — a Kubernetes SRE expert embedded in KubeWatcher.

Your sole responsibility is **root cause analysis** for cluster alerts and incidents.

You have two sub-agents at your disposal:
- **LogAnalysisAgent** — deep-dives logs for error signatures
- **K8sStateAgent** — analyses pod/node state for anomalies

Rules:
1. Given alert info, cluster metrics, and recent events — identify the likely root cause.
2. Assess blast radius: workloads, namespaces, services affected.
3. Distinguish symptoms from causes.
4. Reference specific resources (pod names, nodes, namespaces) when available.
5. If data is insufficient, state what additional information you need.
"""

    def diagnose(self, alert_description: str, cluster_context: str | None = None, logs: str | None = None) -> DiagnosisOutput:
        pipeline = self.sub_agents()

        if logs:
            pipeline.run(LogAnalysisAgent, prompt=f"Analyze these logs:\n{logs}")
        if cluster_context:
            pipeline.run(K8sStateAgent, prompt=f"Analyze this cluster state:\n{cluster_context}")

        enriched = alert_description
        sub_results = pipeline.merge_results()
        if sub_results:
            enriched = f"Alert: {alert_description}\n\nSub-agent findings:\n{sub_results}"

        result = self.structured_chat(
            [{"role": "user", "content": f"Investigate this alert and provide structured diagnosis:\n{enriched}"}],
            DiagnosisOutput,
        )
        self.emit(EVENT_DIAGNOSIS_COMPLETE, {
            "root_cause": result.likely_root_cause,
            "severity": result.severity,
            "confidence": result.confidence,
            "blast_radius": result.blast_radius,
        })
        return result

    def diagnose_freeform(self, alert_description: str, cluster_context: str | None = None) -> str:
        messages = []
        if cluster_context:
            messages.append({"role": "user", "content": f"Current cluster context:\n{cluster_context}"})
        messages.append({"role": "user", "content": f"Investigate this alert:\n{alert_description}"})
        return self.chat(messages)
