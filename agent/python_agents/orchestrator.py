from __future__ import annotations

import importlib
import logging
import time
from datetime import datetime
from typing import Any

from .analysis_agent import AnalysisAgent, MetricsAgent, TrendAgent
from .base import BaseAgent, AgentConfig
from .context_agent import ContextAgent
from .cost_agent import CostAgent, PricingAgent, ResourceUsageAgent
from .diagnosis_agent import DiagnosisAgent, K8sStateAgent, LogAnalysisAgent
from .docs_agent import DocsAgent, OutlineAgent
from .event_bus import (
    BUILTIN_SPAWN_RULES,
    EVENT_ANALYSIS_COMPLETE,
    EVENT_COST_ANALYSIS_COMPLETE,
    EVENT_DIAGNOSIS_COMPLETE,
    EVENT_DOCS_GENERATED,
    EVENT_REMEDIATION_COMPLETE,
    EVENT_REMEDIATION_STARTED,
    EVENT_SECURITY_SCAN_COMPLETE,
    EventBus,
    AgentEvent,
)
from .models import (
    ActionType,
    AgentResult,
    ProactiveInsight,
    SessionContext,
    TaskClassification,
    UserAction,
)
from .proactive_agent import ProactiveAgent
from .remediation_agent import RemediationAgent, RollbackAgent
from .security_agent import SecurityAgent, CveLookupAgent, RbacAuditAgent

logger = logging.getLogger(__name__)

TASK_ROUTING: dict[str, type[BaseAgent]] = {
    "diagnose": DiagnosisAgent,
    "investigate": DiagnosisAgent,
    "root_cause": DiagnosisAgent,
    "remediate": RemediationAgent,
    "fix": RemediationAgent,
    "rollback": RemediationAgent,
    "analyze": AnalysisAgent,
    "health": AnalysisAgent,
    "performance": AnalysisAgent,
    "secure": SecurityAgent,
    "security": SecurityAgent,
    "vuln": SecurityAgent,
    "cost": CostAgent,
    "finops": CostAgent,
    "optimize": CostAgent,
    "docs": DocsAgent,
    "runbook": DocsAgent,
    "postmortem": DocsAgent,
    "notebook": DocsAgent,
}

# ── Event → Sub-agent mapping ────────────────────────────────────
# Maps event_type → (sub-agent class, description)
# These are the "when X fires, spawn Y" rules. Rules with sub-agents
# that don't exist yet are accumulated in _pending_rules and logged.

EVENT_SPAWN_RULES: dict[str, list[tuple[type[BaseAgent], str]]] = {
    EVENT_DIAGNOSIS_COMPLETE: [
        (LogAnalysisAgent, "deep-dive log correlation for diagnosis evidence"),
        (K8sStateAgent, "state anomaly correlation for diagnosis"),
    ],
    EVENT_ANALYSIS_COMPLETE: [
        (TrendAgent, "time-series pattern detection on metrics"),
    ],
    EVENT_REMEDIATION_STARTED: [
        (RollbackAgent, "rollback strategy planning"),
    ],
    EVENT_REMEDIATION_COMPLETE: [
        (RollbackAgent, "post-remediation rollback verification"),
    ],
    EVENT_SECURITY_SCAN_COMPLETE: [
        (RbacAuditAgent, "deep-dive RBAC review triggered by security scan"),
        (CveLookupAgent, "CVE enrichment for scan findings"),
    ],
    EVENT_COST_ANALYSIS_COMPLETE: [
        (PricingAgent, "rate optimisation deep-dive"),
        (ResourceUsageAgent, "resource utilisation cross-check"),
    ],
    EVENT_DOCS_GENERATED: [
        (OutlineAgent, "document structure validation"),
    ],
}


def _make_spawn_handler(
    agent_cls: type[BaseAgent],
    description: str,
    agent_cache: dict[str, BaseAgent],
) -> Any:
    """Factory: returns an EventHandler that spawns the given sub-agent
    with the event payload as context."""

    def handler(event: AgentEvent) -> str | None:
        name = agent_cls.__name__
        if name not in agent_cache:
            logger.debug("bus: spawning %s for '%s'", name, event.event_type)
            agent = agent_cls()
            agent_cache[name] = agent
        agent = agent_cache[name]

        try:
            payload_text = "\n".join(f"{k}: {v}" for k, v in event.payload.items())
            result = agent.chat([
                {"role": "user", "content": (
                    f"Auto-spawned by {event.source_agent} on event '{event.event_type}'.\n"
                    f"Context:\n{payload_text}\n\n"
                    f"Task: {description}"
                )}
            ])
            logger.info("bus: %s completed for '%s' (%d chars)", name, event.event_type, len(result))
            return f"[{name}] {result[:200]}"
        except Exception as e:
            logger.warning("bus: %s failed for '%s': %s", name, event.event_type, e)
            return None

    return handler


class OrchestratorAgent(BaseAgent):
    """Top-level agent that routes user requests, manages context,
    and runs the proactive background loop. Creates the EventBus and
    wires all auto-spawn rules."""

    def __init__(self, config: AgentConfig | None = None):
        super().__init__(config)
        self._agents: dict[str, BaseAgent] = {}
        self.context: ContextAgent = ContextAgent(config)
        self.proactive: ProactiveAgent = ProactiveAgent(config)
        self._insight_buffer: list[ProactiveInsight] = []
        self.bus = EventBus()
        self._register_spawn_rules()

    @property
    def system_prompt(self) -> str:
        return """You are the Argus Orchestrator — the entry point for the Argus multi-agent system.

Your role:
1. Classify the user's request into a task category.
2. Route it to the correct specialist agent.
3. If the request spans multiple domains, break it down and delegate sequentially.
4. Synthesise results from multiple agents into a coherent response.

Available agents:
- **DiagnosisAgent** — root cause analysis for alerts and incidents
- **RemediationAgent** — actionable fix steps and kubectl commands
- **AnalysisAgent** — cluster health and performance metrics analysis
- **SecurityAgent** — vulnerability scanning and security posture
- **CostAgent** — FinOps and cost optimisation
- **DocsAgent** — runbook and postmortem generation
"""

    # ── EventBus wiring ───────────────────────────────────────────

    def _register_spawn_rules(self) -> None:
        """Register all built-in auto-spawn rules on the EventBus."""
        registered = 0
        for event_type, rules in EVENT_SPAWN_RULES.items():
            for agent_cls, description in rules:
                handler = _make_spawn_handler(agent_cls, description, self._agents)
                self.bus.on(event_type, handler)
                registered += 1
        logger.info("bus: registered %d auto-spawn rules across %d event types",
                     registered, len(EVENT_SPAWN_RULES))

    # ── Session management ────────────────────────────────────────

    def start_session(self, user_id: str | None = None) -> SessionContext:
        ctx = self.context.create_session(user_id)
        self.session = ctx
        self.proactive.start(callback=self._on_proactive_insight)
        logger.info("session started: %s", ctx.session_id[:8])
        return ctx

    def stop_session(self) -> None:
        self.proactive.stop()
        logger.info("session stopped")

    # ── Agent registry ────────────────────────────────────────────

    def _get_agent(self, agent_type: type[BaseAgent]) -> BaseAgent:
        name = agent_type.__name__
        if name not in self._agents:
            agent = agent_type(self.config)
            agent.session = self.session
            agent.bus = self.bus
            self._agents[name] = agent
        return self._agents[name]

    # ── Proactive insight flow ────────────────────────────────────

    def _on_proactive_insight(self, insight: ProactiveInsight) -> None:
        self._insight_buffer.append(insight)
        logger.info("proactive insight: %s (conf=%.2f)", insight.suggestion[:50], insight.confidence)

    def pending_proactive(self, clear: bool = True) -> list[ProactiveInsight]:
        results = list(self._insight_buffer)
        if clear:
            self._insight_buffer.clear()
        return results

    # ── Classification ────────────────────────────────────────────

    def _classify(self, user_input: str) -> TaskClassification:
        messages = [{"role": "user", "content": user_input}]
        return self.structured_chat(messages, TaskClassification)

    def classify_and_route(self, user_input: str, action_type: ActionType | None = None) -> AgentResult:
        start = time.monotonic()

        self.context.touch_session()

        if user_input.strip():
            self.context.track_message("user", user_input)

        classification = self._classify(user_input)
        logger.info("classified '%s' as %s (%.0f%%)", user_input[:40], classification.task_type, classification.confidence * 100)

        action = UserAction(
            action_type=action_type or ActionType.query,
            target=classification.task_type,
            namespace=self.session.current_namespace if self.session else None,
            duration_ms=None,
        )
        self.context.track_action(action)
        self.proactive.observe(action)

        recall = self.context.recall(user_input, n_results=3) if self.session else ""

        agent_cls = TASK_ROUTING.get(classification.task_type)
        auto_spawned: list[str] = []

        if agent_cls is None:
            content = f"I couldn't classify this as a known task type (detected: `{classification.task_type}`). Available tasks: diagnose, remediate, analyze, secure, cost, docs."
        else:
            agent = self._get_agent(agent_cls)
            messages = agent.build_messages(user_input, context=recall or None)
            content = agent.chat(messages)
            self.context.track_message("assistant", content)

            # Collect any auto-spawn results triggered by events from this agent
            for event_type in list(EVENT_SPAWN_RULES.keys()):
                results = self.bus.results_for(event_type, clear=True)
                for r in results:
                    spawned_name = r.split("]")[0].strip(" [") if "]" in r else "sub-agent"
                    auto_spawned.append(spawned_name)

        result = AgentResult(
            agent_name=self.agent_name,
            content=content,
            confidence=classification.confidence,
            duration_ms=int((time.monotonic() - start) * 1000),
            auto_spawned=auto_spawned,
        )
        return result

    # ── Route with proactive overlay ──────────────────────────────

    def route(self, user_input: str, context: str | None = None) -> str:
        result = self.classify_and_route(user_input)
        insights = self.pending_proactive(clear=False)

        output = result.content

        if result.auto_spawned:
            output += "\n\n**Auto-spawned analysis completed:**\n"
            for s in result.auto_spawned:
                output += f"  · {s}\n"

        if insights:
            pro_lines: list[str] = []
            for ins in insights[-3:]:
                if not ins.requires_approval:
                    pro_lines.append(f"\n{ins.suggestion}")
            if pro_lines:
                output += "\n\n" + "".join(pro_lines)

        return output

    # ── Event introspection ───────────────────────────────────────

    def registered_events(self) -> list[str]:
        """Which event types have handlers registered."""
        return self.bus.registered_events()

    def bus_summary(self) -> str:
        """Human-readable summary of EventBus state."""
        events = self.registered_events()
        lines = [f"EventBus: {len(events)} event types with handlers"]
        for e in sorted(events):
            count = len(self.bus.handlers_for(e))
            lines.append(f"  · {e} ({count} handler{'s' if count > 1 else ''})")
        return "\n".join(lines)

    # ── Activity report ───────────────────────────────────────────

    def activity_report(self) -> str:
        """What the system is doing right now — makes it feel alive."""
        lines: list[str] = []
        lines.append("── Argus Activity Report ──")

        if self.session:
            actions = len(self.session.recent_actions)
            msgs = len(self.session.conversation)
            dur = int((datetime.now() - self.session.start_time).total_seconds() // 60)
            lines.append(f"Session: {dur}m active, {actions} actions, {msgs} messages")

        pro_report = self.proactive.report_activity()
        if pro_report:
            lines.append("")
            lines.append(pro_report)

        insights = self.pending_proactive(clear=False)
        if insights:
            lines.append("")
            lines.append("  Pre-fetching / pre-computing:")
            for ins in insights:
                icon = "⚡" if ins.auto_executed else "💡"
                lines.append(f"    {icon} {ins.suggestion}")

        # EventBus state
        events = self.registered_events()
        if events:
            lines.append("")
            lines.append(f"  Event bus: {len(events)} active event types")

        lines.append("")
        lines.append(f"  Memory store: {self.context.store.count('memories')} items | {self.context.store.count('patterns')} patterns")
        lines.append("── ──────────── ──")

        return "\n".join(lines)

    # ── Lifecycle ─────────────────────────────────────────────────

    def close(self) -> None:
        self.proactive.close()
        for agent in self._agents.values():
            agent.close()
        self.context.close()
        self.bus.clear()
        super().close()
