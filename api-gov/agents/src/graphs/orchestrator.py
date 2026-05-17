from __future__ import annotations

import json
import logging
from datetime import datetime, timezone

from langgraph.graph import END, StateGraph
from langgraph.checkpoint.memory import MemorySaver
from langgraph.checkpoint.redis import RedisSaver

from src.config import config
from src.graphs.state import AgentState
from src.redis_client import redis_client

logger = logging.getLogger(__name__)

SPOT_CHECK_PROMPT = """You are a Quality Gate. Review the agent's output and check for:
1. Over-engineering (complexity score 0-100, >60 = fail)
2. Missing edge cases
3. Hallucinated fields not in the original spec
4. Unnecessary changes

Return JSON:
{{"passed": bool, "complexity_score": int (0-100), "feedback": [str], "refinement_needed": bool}}

Agent output:
{output}
"""


async def load_context(state: AgentState) -> dict:
    from src.database import db
    endpoints = await db.fetch_endpoints(state.spec_id)
    spec = await db.fetch_spec(state.spec_id)
    profile = await redis_client.get_profile(f"user:{state.spec_id}")
    return {
        "endpoints": endpoints,
        "spec_content": spec.get("content") if spec else None,
        "user_profile": profile or {"skill_level": "intermediate", "session_count": 0},
    }


async def profile_user(state: AgentState) -> dict:
    profile = state.user_profile or {}
    profile["session_count"] = profile.get("session_count", 0) + 1
    profile["last_active"] = datetime.now(timezone.utc).isoformat()
    await redis_client.update_profile(f"user:{state.spec_id}", profile)
    return {"user_profile": profile}


async def dispatch_subgraph(state: AgentState) -> dict:
    action = state.messages[0].get("action", "analyze") if state.messages else "analyze"
    result = {}

    if action == "analyze":
        from src.graphs.architect import analyze
        result["analysis"] = await analyze(state.spec_id)
    elif action == "scan":
        from src.graphs.sentinel import scan
        result["drift_reports"] = await scan(state.spec_id)
    elif action == "generate_tests":
        from src.graphs.hacker import generate
        endpoint_id = state.messages[0].get("endpoint_id", "") if state.messages else ""
        result["test_cases"] = await generate(state.spec_id, endpoint_id)
    elif action == "investigate":
        from src.graphs.investigator import investigate
        drift_report = state.messages[0].get("drift_report", {}) if state.messages else {}
        result["investigation"] = await investigate(state.spec_id, drift_report)

    return result


async def collect_results(state: AgentState) -> dict:
    return {}


async def spot_check(state: AgentState) -> dict:
    output = {}
    if state.analysis:
        output["analysis"] = state.analysis
    if state.drift_reports:
        output["drift_reports"] = state.drift_reports[:3]
    if state.test_cases:
        output["test_cases"] = state.test_cases[:3]

    if not output:
        return {"spot_check_passed": True}

    llm = config.create_llm(temperature=0.0)
    prompt = SPOT_CHECK_PROMPT.format(output=json.dumps(output, indent=2, default=str))
    response = await llm.ainvoke(prompt)

    try:
        content = response.content if hasattr(response, "content") else str(response)
        check = json.loads(content.strip().removeprefix("```json").removeprefix("```").removesuffix("```"))
    except (json.JSONDecodeError, AttributeError):
        check = {"passed": True, "complexity_score": 0, "feedback": [], "refinement_needed": False}

    return {"spot_check_passed": check.get("passed", True)}


async def refine(state: AgentState) -> dict:
    state.refinement_count += 1
    if state.refinement_count < 3:
        logger.info("refining output (attempt %d/3)", state.refinement_count)
        return await dispatch_subgraph(state)
    return {}


def build_orchestrator_graph(redis_url: str | None = None) -> StateGraph:
    workflow = StateGraph(AgentState)
    workflow.add_node("load_context", load_context)
    workflow.add_node("profile_user", profile_user)
    workflow.add_node("dispatch_subgraph", dispatch_subgraph)
    workflow.add_node("collect_results", collect_results)
    workflow.add_node("spot_check", spot_check)
    workflow.add_node("refine", refine)
    workflow.set_entry_point("load_context")
    workflow.add_edge("load_context", "profile_user")
    workflow.add_edge("profile_user", "dispatch_subgraph")
    workflow.add_edge("dispatch_subgraph", "collect_results")
    workflow.add_edge("collect_results", "spot_check")
    workflow.add_conditional_edges(
        "spot_check",
        lambda s: "refine" if not s.spot_check_passed and s.refinement_count < 3 else END,
    )
    workflow.add_edge("refine", "collect_results")

    if redis_url:
        checkpointer = RedisSaver(redis_url)
    else:
        checkpointer = MemorySaver()
    return workflow.compile(checkpointer=checkpointer)


orchestrator_graph = build_orchestrator_graph(redis_url=config.redis_url)


async def orchestrate(spec_id: str, action: str = "analyze", endpoint_id: str | None = None, drift_report: dict | None = None) -> dict:
    initial = AgentState(
        spec_id=spec_id,
        messages=[{"action": action, "endpoint_id": endpoint_id or "", "drift_report": drift_report or {}}],
    )
    result = await orchestrator_graph.ainvoke(
        initial,
        config={"configurable": {"thread_id": f"orchestrator-{spec_id}-{action}"}},
    )
    return {
        "analysis": result.get("analysis"),
        "drift_reports": result.get("drift_reports"),
        "test_cases": result.get("test_cases"),
        "investigation": result.get("investigation"),
        "spot_check_passed": result.get("spot_check_passed", True),
    }
