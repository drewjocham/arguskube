from __future__ import annotations

from pydantic import BaseModel, Field

from argus_agents.base import BaseAgent
from argus_agents.event_bus import EVENT_DOCS_GENERATED


class RunbookSection(BaseModel):
    title: str
    content: str
    commands: list[str] = Field(default_factory=list)


class RunbookOutput(BaseModel):
    title: str
    alert_type: str
    preflight_checks: list[str]
    sections: list[RunbookSection]
    rollback_steps: list[str]
    references: list[str] = Field(default_factory=list)


class PostmortemOutput(BaseModel):
    title: str
    incident_id: str = ""
    date: str = ""
    summary: str
    timeline: list[str]
    root_cause: str
    impact: str
    action_items: list[str]
    lessons_learned: list[str]


class OutlineAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return "You are a documentation outline sub-agent. Given a topic, produce a structured outline with sections and subsections."

    def generate(self, topic: str, doc_type: str) -> str:
        return self.chat([{"role": "user", "content": f"Create an outline for a {doc_type} on: {topic}"}])


class DocsAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are the Argus Documentation Agent — a technical writer for Kubernetes operations.

Your sole responsibility is **generating and maintaining operational documentation**.

Types:
1. **Runbooks** — step-by-step incident response guides for specific alert types.
2. **Postmortems** — structured incident reviews with timeline, root cause, action items.
3. **Notebooks** — cluster architecture notes, topology descriptions, operational recipes.
4. **SRE Reports** — weekly/monthly summaries of cluster health, incidents, and changes.

Rules:
1. Write in clear, imperative English. Assume the reader is an on-call SRE.
2. Use markdown with consistent heading structure.
3. Include executable kubectl commands in code blocks.
4. For runbooks: pre-flight checks, remediation, and rollback.
5. For postmortems: blameless language, focus on system failures.
"""

    def generate_runbook(self, alert_type: str, context: str | None = None) -> RunbookOutput:
        pipeline = self.sub_agents()
        pipeline.run(OutlineAgent, prompt=f"Create outline for runbook on '{alert_type}' alerts", doc_type="runbook")

        enriched = f"Alert type: {alert_type}"
        sub_results = pipeline.merge_results()
        if sub_results:
            enriched = f"{enriched}\n\nOutline:\n{sub_results}"
        if context:
            enriched = f"{enriched}\n\nContext: {context}"

        result = self.structured_chat(
            [{"role": "user", "content": f"Create a structured runbook:\n{enriched}"}],
            RunbookOutput,
        )
        self.emit(EVENT_DOCS_GENERATED, {
            "title": result.title,
            "alert_type": result.alert_type,
            "section_count": len(result.sections),
        })
        return result

    def generate_postmortem(self, incident_summary: str, timeline: str, impact: str) -> PostmortemOutput:
        result = self.structured_chat(
            [{"role": "user", "content": f"Write a blameless postmortem:\nSummary: {incident_summary}\nTimeline: {timeline}\nImpact: {impact}"}],
            PostmortemOutput,
        )
        self.emit(EVENT_DOCS_GENERATED, {
            "title": result.title,
            "type": "postmortem",
            "action_items": len(result.action_items),
        })
        return result

    def generate_runbook_freeform(self, alert_type: str, context: str | None = None) -> str:
        return self.chat([
            {"role": "user", "content": f"Create a runbook for handling '{alert_type}' alerts.\n{context or ''}"}
        ])

    def generate_postmortem_freeform(self, incident_summary: str, timeline: str, impact: str) -> str:
        return self.chat([
            {"role": "user", "content":
                f"Write a blameless postmortem for this incident:\n\nSummary: {incident_summary}\n\nTimeline:\n{timeline}\n\nImpact:\n{impact}"}
        ])
