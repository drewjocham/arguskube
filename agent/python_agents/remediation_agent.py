from __future__ import annotations

from pydantic import BaseModel, Field

from .base import BaseAgent
from .event_bus import EVENT_REMEDIATION_COMPLETE


class RemediationStep(BaseModel):
    priority: str = "P2"  # P0, P1, P2
    action: str
    command: str = ""
    verification: str = ""
    side_effects: str = ""
    rollback: str = ""


class RemediationOutput(BaseModel):
    diagnosis_summary: str
    immediate_action: str
    steps: list[RemediationStep]
    verification_commands: list[str]
    rollback_plan: str = ""
    risk_level: str = "low"


class RollbackAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are a rollback planning sub-agent. Given a current state and a problem,
determine the safest rollback strategy. Include exact revision targets and
kubectl commands."""

    def plan(self, diagnosis: str) -> str:
        return self.chat([{"role": "user", "content": f"Plan a rollback for this issue:\n{diagnosis}"}])


class RemediationAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are the Argus Remediation Agent — an action-oriented Kubernetes SRE assistant.

Your sole responsibility is **generating actionable remediation steps** for cluster issues.

Rules:
1. Order steps by severity: stop the bleeding first, then fix, then restore.
2. Always include ready-to-paste kubectl commands.
3. Include validation commands to confirm the fix.
4. Warn about side effects.
5. Prefer non-disruptive actions first.
6. Suggest the safest default when uncertain.
"""

    def remediate(self, diagnosis: str, issue_context: str | None = None) -> RemediationOutput:
        pipeline = self.sub_agents()

        if "rollback" in diagnosis.lower() or "deploy" in diagnosis.lower():
            pipeline.run(RollbackAgent, prompt=f"Plan rollback for:\n{diagnosis}")

        enriched = diagnosis
        sub_results = pipeline.merge_results()
        if sub_results:
            enriched = f"{diagnosis}\n\nRollback analysis:\n{sub_results}"

        if issue_context:
            enriched = f"Context: {issue_context}\n\n{enriched}"

        result = self.structured_chat(
            [{"role": "user", "content": f"Given this diagnosis, what remediation steps should be taken?\n{enriched}"}],
            RemediationOutput,
        )
        self.emit(EVENT_REMEDIATION_COMPLETE, {
            "action": result.immediate_action,
            "risk": result.risk_level,
            "step_count": len(result.steps),
        })
        return result

    def remediate_freeform(self, diagnosis: str, issue_context: str | None = None) -> str:
        messages = []
        if issue_context:
            messages.append({"role": "user", "content": f"Context:\n{issue_context}"})
        messages.append({"role": "user", "content": f"Given this diagnosis, what actions should be taken?\n{diagnosis}"})
        return self.chat(messages)
