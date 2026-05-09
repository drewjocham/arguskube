from __future__ import annotations

import logging
import threading
import time
import uuid
from collections import defaultdict
from datetime import datetime
from typing import Any, Callable

from pydantic import BaseModel, Field

from .models import AgentResult

logger = logging.getLogger(__name__)


class AgentEvent(BaseModel):
    """A typed event emitted by an agent during its lifecycle."""

    event_type: str
    source_agent: str
    session_id: str = ""
    correlation_id: str = Field(default_factory=lambda: str(uuid.uuid4())[:8])
    payload: dict[str, Any] = Field(default_factory=dict)
    timestamp: datetime = Field(default_factory=datetime.now)


EventHandler = Callable[[AgentEvent], str | None]


class EventBus:
    """Lightweight pub-sub event bus for agent-to-agent communication.

    Agents emit typed events. Handlers registered via ``on()`` are called
    synchronously on ``emit()`` (or in a background thread via ``emit_async()``).

    Handler results are stored on the event for introspection. This keeps
    agents decoupled — no agent imports or knows about the sub-agent it spawns.
    """

    def __init__(self):
        self._handlers: dict[str, list[EventHandler]] = defaultdict(list)
        self._lock = threading.Lock()
        self._results: dict[str, list[str]] = defaultdict(list)

    # ── Registration ─────────────────────────────────────────────

    def on(self, event_type: str, handler: EventHandler) -> None:
        """Register a handler for an event type. Handlers receive the
        ``AgentEvent`` and should return a string summary or None."""
        with self._lock:
            self._handlers[event_type].append(handler)
        logger.debug("bus: registered handler for '%s'", event_type)

    def on_many(self, event_types: list[str], handler: EventHandler) -> None:
        """Register the same handler for multiple event types."""
        for et in event_types:
            self.on(et, handler)

    def handlers_for(self, event_type: str) -> list[EventHandler]:
        """Return all handlers registered for an event type."""
        with self._lock:
            return list(self._handlers.get(event_type, []))

    # ── Emission ──────────────────────────────────────────────────

    def emit(
        self,
        event_type: str,
        source: str = "system",
        payload: dict[str, Any] | None = None,
        correlation_id: str | None = None,
    ) -> AgentEvent:
        """Emit an event synchronously. All handlers run in the calling thread.
        Returns the ``AgentEvent`` with handler results populated."""
        event = AgentEvent(
            event_type=event_type,
            source_agent=source,
            payload=payload or {},
            correlation_id=correlation_id or str(uuid.uuid4())[:8],
        )

        handlers = self.handlers_for(event_type)
        if not handlers:
            event.payload.setdefault("_results", [])
            return event

        results: list[str] = []
        for handler in handlers:
            try:
                result = handler(event)
                if result is not None:
                    results.append(result)
            except Exception as e:
                logger.warning("bus: handler for '%s' failed: %s", event_type, e)

        event.payload["_results"] = results
        with self._lock:
            self._results[event_type].extend(results)

        return event

    def emit_async(
        self,
        event_type: str,
        source: str = "system",
        payload: dict[str, Any] | None = None,
        correlation_id: str | None = None,
    ) -> AgentEvent:
        """Emit an event in a daemon background thread. Returns immediately
        with the event. Handler results are stored on the bus for later retrieval."""
        event = AgentEvent(
            event_type=event_type,
            source_agent=source,
            payload=payload or {},
            correlation_id=correlation_id or str(uuid.uuid4())[:8],
        )

        handlers = self.handlers_for(event_type)
        if handlers:
            t = threading.Thread(target=self._run_async_handlers, args=(event, handlers), daemon=True)
            t.start()

        return event

    def _run_async_handlers(self, event: AgentEvent, handlers: list[EventHandler]) -> None:
        results: list[str] = []
        for handler in handlers:
            try:
                result = handler(event)
                if result is not None:
                    results.append(result)
            except Exception as e:
                logger.warning("bus: async handler for '%s' failed: %s", event.event_type, e)

        with self._lock:
            self._results[event.event_type].extend(results)

    # ── Introspection ─────────────────────────────────────────────

    def results_for(self, event_type: str, clear: bool = True) -> list[str]:
        """Get all handler result summaries for an event type."""
        with self._lock:
            results = list(self._results.get(event_type, []))
            if clear:
                self._results[event_type] = []
            return results

    def registered_events(self) -> list[str]:
        """List all event types that have at least one handler."""
        with self._lock:
            return list(self._handlers.keys())

    def clear(self) -> None:
        """Remove all handlers and results."""
        with self._lock:
            self._handlers.clear()
            self._results.clear()


# ── Standard event type constants ─────────────────────────────────

EVENT_ALERT_DETECTED = "agent.alert.detected"
EVENT_DIAGNOSIS_STARTED = "agent.diagnosis.started"
EVENT_DIAGNOSIS_COMPLETE = "agent.diagnosis.complete"
EVENT_REMEDIATION_STARTED = "agent.remediation.started"
EVENT_REMEDIATION_COMPLETE = "agent.remediation.complete"
EVENT_ANALYSIS_STARTED = "agent.analysis.started"
EVENT_ANALYSIS_COMPLETE = "agent.analysis.complete"
EVENT_SECURITY_SCAN_STARTED = "agent.security.scan.started"
EVENT_SECURITY_SCAN_COMPLETE = "agent.security.scan.complete"
EVENT_COST_ANALYSIS_STARTED = "agent.cost.analysis.started"
EVENT_COST_ANALYSIS_COMPLETE = "agent.cost.analysis.complete"
EVENT_DOCS_GENERATED = "agent.docs.generated"
EVENT_USER_QUERY = "agent.user.query"
EVENT_PROACTIVE_INSIGHT = "agent.proactive.insight"
EVENT_ERROR = "agent.error"

# ── Event-based sub-agent spawners ───────────────────────────────

# Maps: event_type → (handler that spawns sub-agent, description)
# Registered by the Orchestrator on startup. These are the "auto-spawn" rules.
BUILTIN_SPAWN_RULES: list[tuple[str, str, str]] = [
    # When a diagnosis completes, auto-spawn deep-dive log analysis
    (EVENT_DIAGNOSIS_COMPLETE,
     "spawn LogAnalysisAgent for deep-dive log correlation",
     "diagnosis_agent.LogAnalysisAgent"),
    # When analysis completes, auto-spawn trend detection
    (EVENT_ANALYSIS_COMPLETE,
     "spawn TrendAgent for time-series pattern detection",
     "analysis_agent.TrendAgent"),
    # When a remediation starts, auto-spawn rollback planning
    (EVENT_REMEDIATION_STARTED,
     "spawn RollbackAgent for rollback strategy",
     "remediation_agent.RollbackAgent"),
    # When a security scan completes, auto-spawn RBAC audit
    (EVENT_SECURITY_SCAN_COMPLETE,
     "spawn RbacAuditAgent for deep-dive RBAC review",
     "security_agent.RbacAuditAgent"),
    # When a cost analysis completes, auto-spawn pricing deep-dive
    (EVENT_COST_ANALYSIS_COMPLETE,
     "spawn PricingAgent for rate optimisation",
     "cost_agent.PricingAgent"),
    # When docs are generated, auto-spawn outline verification
    (EVENT_DOCS_GENERATED,
     "spawn OutlineAgent for structure validation",
     "docs_agent.OutlineAgent"),
]
