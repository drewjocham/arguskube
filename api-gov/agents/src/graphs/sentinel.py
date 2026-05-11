from __future__ import annotations

import json
import logging
from typing import Any

from langgraph.graph import END, StateGraph
from langgraph.checkpoint.memory import MemorySaver
from langgraph.checkpoint.redis import RedisSaver

from src.anomaly.api_anomaly import APIDriftDetector
from src.anomaly.counters import AnomalyCounters
from src.anomaly.metrics_anomaly import MetricsAnomalyDetector
from src.anomaly.structural import FieldTracker
from src.config import config
from src.graphs.state import AgentState

logger = logging.getLogger(__name__)

DRIFT_PROMPT = """You are a Sentinel detecting API drift (Layer 3 — ambiguous cases only).
The previous layers found potential issues. Review them and decide if they are real drifts or false positives.

Potential issues:
{issues}

Endpoint: {method} {path}
Status: {status}
Return a JSON array of confirmed drift reports.
"""


async def record_counters(state: AgentState) -> dict:
    """Layer 0: Record counters only — zero analysis, zero storage of raw data."""
    event = state.messages[0] if state.messages else state.spec_content or {}
    method = event.get("method", "GET")
    path = event.get("path", "/")
    status = event.get("status_code", 200)
    latency = event.get("latency_ms", float(event.get("latency_ms", 50.0)))

    await AnomalyCounters.record_event(
        spec_id=state.spec_id,
        method=method,
        path=path,
        status=status,
        latency_ms=latency,
    )
    return {"messages": []}


async def structural_check(state: AgentState) -> dict:
    """Layer 1: Statistical + structural checks — no LLM. Runs on every event."""
    event = state.messages[0] if state.messages else {}
    if not event:
        return {"drift_reports": [], "errors": []}

    method = event.get("method", "GET")
    path = event.get("path", "/")
    status = event.get("status_code", 200)
    latency = event.get("latency_ms", 50.0)
    response = event.get("response") or {}
    request = event.get("request") or {}

    reports: list[dict] = []

    # 1.1 — API field drift (if we have response body fields)
    if response:
        observed_fields = list(response.keys()) if isinstance(response, dict) else []
        defined_fields = _get_defined_fields(state.endpoints, method, path)
        if observed_fields:
            field_reports = await APIDriftDetector.detect_field_drift(
                state.spec_id, method, path, observed_fields, defined_fields,
            )
            reports.extend(field_reports)
            await FieldTracker.record_fields(state.spec_id, method, path, observed_fields)

    # 1.2 — Status anomaly
    status_report = await APIDriftDetector.detect_status_anomaly(state.spec_id, status, method, path)
    if status_report:
        reports.append(status_report)

    # 1.3 — Response size drift (schema change proxy)
    response_size = len(json.dumps(response)) if response else 0
    size_report = await APIDriftDetector.check_schema_drift(state.spec_id, method, path, "application/json", response_size)
    if size_report:
        reports.append(size_report)

    # 1.4 — Metrics anomalies (latency, throughput, error rate)
    metrics_reports = await MetricsAnomalyDetector.detect_all(state.spec_id, method, path, status, latency)
    reports.extend(metrics_reports)

    return {"drift_reports": reports}


async def llm_verify(state: AgentState) -> dict:
    """Layer 2: Write ambiguous drift candidates to JSONL batch files.
    The batch worker will pick these up and call LLM once per spec per 5min window.
    No inline LLM calls — cost reduced by ~95% via batching + dedup.
    """
    if not state.drift_reports:
        return {}

    # Only batch for field-level drifts (structural is confident on status/metrics)
    ambiguous = [r for r in state.drift_reports if r.get("category") in ("undocumented_field", "missing_field")]
    if not ambiguous:
        return {}

    from src.anomaly.metadata_filter import MetadataFilter
    from src.anomaly.batch_worker import write_candidate

    for candidate in ambiguous:
        # Skip if this exact pattern was recently assessed
        known = await MetadataFilter.is_known(
            state.spec_id,
            candidate.get("method", "GET"),
            candidate.get("path", "/"),
            candidate.get("field", ""),
            candidate.get("category", "unknown"),
        )
        if not known:
            await write_candidate(state.spec_id, {**candidate, "spec_id": state.spec_id})

    # Non-ambiguous reports pass through directly (no LLM needed)
    final = [r for r in state.drift_reports if r.get("category") not in ("undocumented_field", "missing_field")]
    return {"drift_reports": final}


async def score_and_persist(state: AgentState) -> dict:
    """Score and persist significant drifts."""
    significant = [r for r in state.drift_reports if r.get("score", 1.0) < config.drift_threshold]
    if significant:
        from src.database import db
        await db.save_drift_reports(significant)
        logger.info("persisted %d drift reports", len(significant))
    return {"drift_reports": significant}


def _get_defined_fields(endpoints: list[dict], method: str, path: str) -> list[str]:
    for ep in endpoints:
        if ep.get("method", "").upper() == method.upper() and ep.get("path") == path:
            schema = ep.get("responses", {})
            resp_200 = schema.get("200", schema.get("default", {}))
            if isinstance(resp_200, dict):
                props = resp_200.get("properties", resp_200.get("schema", {}))
                if isinstance(props, dict):
                    return list(props.keys())
    return []


def build_sentinel_graph(redis_url: str | None = None) -> StateGraph:
    workflow = StateGraph(AgentState)
    workflow.add_node("record_counters", record_counters)
    workflow.add_node("structural_check", structural_check)
    workflow.add_node("llm_verify", llm_verify)
    workflow.add_node("score_and_persist", score_and_persist)
    workflow.set_entry_point("record_counters")
    workflow.add_edge("record_counters", "structural_check")
    workflow.add_edge("structural_check", "llm_verify")
    workflow.add_edge("llm_verify", "score_and_persist")
    workflow.add_edge("score_and_persist", END)

    if redis_url:
        checkpointer = RedisSaver(redis_url)
    else:
        checkpointer = MemorySaver()
    return workflow.compile(checkpointer=checkpointer)


sentinel_graph = build_sentinel_graph()


async def ingest(event: dict) -> None:
    spec_id = event.get("spec_id", "")
    initial = AgentState(spec_id=spec_id, messages=[event])
    await sentinel_graph.ainvoke(
        initial,
        config={"configurable": {"thread_id": f"ingest-{spec_id}"}},
    )


async def scan(spec_id: str) -> list[dict]:
    from src.database import db
    endpoints = await db.fetch_endpoints(spec_id)
    initial = AgentState(spec_id=spec_id, endpoints=endpoints)
    result = await sentinel_graph.ainvoke(
        initial,
        config={"configurable": {"thread_id": f"scan-{spec_id}"}},
    )
    return result.get("drift_reports", [])
