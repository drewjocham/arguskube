from __future__ import annotations

import json
import logging

from langgraph.graph import END, StateGraph
from langgraph.checkpoint.memory import MemorySaver
from langgraph.checkpoint.redis import RedisSaver

from src.config import config
from src.graphs.state import AgentState

logger = logging.getLogger(__name__)

HEALER_PROMPT = """You are a Healer Agent. Given a drift report, suggest the exact fix.
Decide if the drift is intentional (API evolved → update spec) or a bug (drift is wrong → fix API).

Return JSON:
{{"spec_update": {{"field": str, "old_value": any, "new_value": any}} | null,
  "test_update": str,
  "confidence": float (0-1),
  "explanation": str,
  "drift_type": "intentional"|"bug"}}

Drift report:
{report}
"""


async def load_report(state: AgentState) -> dict:
    return {}


async def llm_classify_drift(state: AgentState) -> dict:
    if not state.drift_reports:
        return {}

    llm = config.create_llm(temperature=0.0)
    report = state.drift_reports[0]
    report_str = json.dumps(report, indent=2, default=str)
    prompt = HEALER_PROMPT.format(report=report_str)
    response = await llm.ainvoke(prompt)

    try:
        content = response.content if hasattr(response, "content") else str(response)
        advice = json.loads(content.strip().removeprefix("```json").removeprefix("```").removesuffix("```"))
    except (json.JSONDecodeError, AttributeError):
        advice = {
            "spec_update": None,
            "test_update": "Review the drift report manually",
            "confidence": 0.0,
            "explanation": "Could not parse LLM response",
            "drift_type": "bug",
        }

    return {"analysis": {"fix": advice, "report_id": report.get("id", "")}}


async def calculate_confidence(state: AgentState) -> dict:
    fix = (state.analysis or {}).get("fix", {})
    confidence = fix.get("confidence", 0.0)
    if confidence >= 0.8 and fix.get("drift_type") == "intentional":
        fix["auto_apply"] = True
    else:
        fix["auto_apply"] = False
    return {"analysis": {"fix": fix}}


def build_healer_graph(redis_url: str | None = None) -> StateGraph:
    workflow = StateGraph(AgentState)
    workflow.add_node("load_report", load_report)
    workflow.add_node("llm_classify_drift", llm_classify_drift)
    workflow.add_node("calculate_confidence", calculate_confidence)
    workflow.set_entry_point("load_report")
    workflow.add_edge("load_report", "llm_classify_drift")
    workflow.add_edge("llm_classify_drift", "calculate_confidence")
    workflow.add_edge("calculate_confidence", END)

    if redis_url:
        checkpointer = RedisSaver(redis_url)
    else:
        checkpointer = MemorySaver()
    return workflow.compile(checkpointer=checkpointer)


healer_graph = build_healer_graph(redis_url=config.redis_url)


async def heal(report: dict) -> dict:
    initial = AgentState(drift_reports=[report])
    result = await healer_graph.ainvoke(
        initial,
        config={"configurable": {"thread_id": f"heal-{report.get('id', 'unknown')}"}},
    )
    return (result.get("analysis") or {}).get("fix", {})
