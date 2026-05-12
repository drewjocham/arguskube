"""Investigator Agent — root cause analysis on high-severity alerts.

Spawned by Orchestrator when a critical/high alert fires.
Queries metrics, checks recent changes, and produces a root cause hypothesis.
"""

from __future__ import annotations

import json
import logging
from datetime import datetime, timezone

from langgraph.checkpoint.memory import MemorySaver
from langgraph.checkpoint.redis import RedisSaver
from langgraph.graph import END, StateGraph

from src.config import config
from src.graphs.state import AgentState

logger = logging.getLogger(__name__)

INVESTIGATOR_PROMPT = """You are an Investigator analyzing an API drift alert.
Given the drift report, recent metric history, and current spec, determine root cause.

Return:
{{"root_cause": str, "confidence": float (0-1), "is_false_positive": bool,
  "action": "auto_fix_spec"|"auto_fix_api"|"flag_for_review"|"dismiss",
  "explanation": str}}

Drift report:
{report}

Recent metrics:
{metrics}

Current spec summary:
{spec}
"""


async def load_context(state: AgentState) -> dict:
    from src.database import db
    endpoints = await db.fetch_endpoints(state.spec_id)
    spec = await db.fetch_spec(state.spec_id)
    return {"endpoints": endpoints, "spec_content": spec.get("content") if spec else None}


async def llm_investigate(state: AgentState) -> dict:
    if not state.drift_reports and not state.messages:
        return {"analysis": {"result": "nothing_to_investigate", "confidence": 1.0}}

    report = state.drift_reports[0] if state.drift_reports else state.messages[0]
    llm = config.create_llm(temperature=0.0)
    prompt = INVESTIGATOR_PROMPT.format(
        report=json.dumps(report, indent=2, default=str),
        metrics="",  # Would be populated from DuckDB in production
        spec=f"{len(state.endpoints)} endpoints" if state.endpoints else "No spec loaded",
    )
    response = await llm.ainvoke(prompt)
    try:
        content = response.content if hasattr(response, "content") else str(response)
        result = json.loads(content.strip().removeprefix("```json").removeprefix("```").removesuffix("```"))
    except (json.JSONDecodeError, AttributeError):
        result = {
            "root_cause": "Could not determine — LLM parse failed",
            "confidence": 0.0,
            "is_false_positive": False,
            "action": "flag_for_review",
            "explanation": "LLM response could not be parsed",
        }

    if not result.get("action"):
        result["action"] = "flag_for_review"

    return {"analysis": {"investigation": result}}


async def record_result(state: AgentState) -> dict:
    investigation = (state.analysis or {}).get("investigation", {})
    from src.database import db
    await db.save_investigation(
        spec_id=state.spec_id,
        agent_name="investigator",
        result=investigation.get("root_cause", "unknown"),
        confidence=investigation.get("confidence", 0.0),
        log=investigation,
    )
    return {}


def build_investigator_graph(redis_url: str | None = None) -> StateGraph:
    workflow = StateGraph(AgentState)
    workflow.add_node("load_context", load_context)
    workflow.add_node("llm_investigate", llm_investigate)
    workflow.add_node("record_result", record_result)
    workflow.set_entry_point("load_context")
    workflow.add_edge("load_context", "llm_investigate")
    workflow.add_edge("llm_investigate", "record_result")
    workflow.add_edge("record_result", END)

    if redis_url:
        checkpointer = RedisSaver(redis_url)
    else:
        checkpointer = MemorySaver()
    return workflow.compile(checkpointer=checkpointer)


investigator_graph = build_investigator_graph()


async def investigate(spec_id: str, drift_report: dict) -> dict:
    initial = AgentState(spec_id=spec_id, drift_reports=[drift_report])
    result = await investigator_graph.ainvoke(
        initial,
        config={"configurable": {"thread_id": f"investigate-{spec_id}-{drift_report.get('id', 'unknown')}"}},
    )
    return (result.get("analysis") or {}).get("investigation", {})
