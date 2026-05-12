from __future__ import annotations

import json
import logging
from typing import Any

from langgraph.graph import END, StateGraph
from langgraph.checkpoint.memory import MemorySaver
from langgraph.checkpoint.redis import RedisSaver

from src.config import config
from src.graphs.state import AgentState

logger = logging.getLogger(__name__)

ANALYSIS_PROMPT = """You are an API Architect. Analyze the following OpenAPI spec and return a JSON object with exactly these keys:
- "endpoints": total number of API endpoints
- "critical_paths": number of POST/PUT/PATCH/DELETE endpoints
- "auth_routes": number of endpoints with security requirements
- "summary": 2-3 sentence overview of the API

Spec content:
{spec}

Endpoints:
{endpoints}
"""


async def load_spec(state: AgentState) -> dict:
    from src.database import db

    endpoints = await db.fetch_endpoints(state.spec_id)
    spec = await db.fetch_spec(state.spec_id)
    return {"endpoints": endpoints, "spec_content": spec.get("content") if spec else None}


async def extract_metadata(state: AgentState) -> dict:
    if not state.endpoints:
        return {"endpoints": []}
    methods = [ep.get("method", "GET").upper() for ep in state.endpoints]
    return {
        "analysis": {
            "total_endpoints": len(methods),
            "critical_count": sum(1 for m in methods if m in ("POST", "PUT", "PATCH", "DELETE")),
            "auth_count": sum(1 for ep in state.endpoints if ep.get("security")),
            "methods": list(set(methods)),
            "tags": list(set(t for ep in state.endpoints for t in ep.get("tags", []))),
        }
    }


async def llm_classify(state: AgentState) -> dict:
    llm = config.create_llm(temperature=0.0)
    spec_str = json.dumps(state.spec_content, indent=2)[:5000] if state.spec_content else "{}"
    ep_str = json.dumps(state.endpoints, indent=2, default=str)[:3000] if state.endpoints else "[]"

    prompt = ANALYSIS_PROMPT.format(spec=spec_str, endpoints=ep_str)
    response = await llm.ainvoke(prompt)

    try:
        content = response.content if hasattr(response, "content") else str(response)
        analysis = json.loads(content.strip().removeprefix("```json").removeprefix("```").removesuffix("```"))
    except (json.JSONDecodeError, AttributeError):
        meta = state.analysis or {}
        analysis = {
            "endpoints": meta.get("total_endpoints", len(state.endpoints)),
            "critical_paths": meta.get("critical_count", 0),
            "auth_routes": meta.get("auth_count", 0),
            "summary": "Analysis completed via endpoint counting (LLM parse failed)",
        }

    return {"analysis": analysis}


def build_architect_graph(redis_url: str | None = None) -> StateGraph:
    workflow = StateGraph(AgentState)
    workflow.add_node("load_spec", load_spec)
    workflow.add_node("extract_metadata", extract_metadata)
    workflow.add_node("llm_classify", llm_classify)
    workflow.set_entry_point("load_spec")
    workflow.add_edge("load_spec", "extract_metadata")
    workflow.add_edge("extract_metadata", "llm_classify")
    workflow.add_edge("llm_classify", END)

    if redis_url:
        checkpointer = RedisSaver(redis_url)
    else:
        checkpointer = MemorySaver()
    return workflow.compile(checkpointer=checkpointer)


architect_graph = build_architect_graph()


async def analyze(spec_id: str) -> dict:
    initial = AgentState(spec_id=spec_id)
    result = await architect_graph.ainvoke(
        initial,
        config={"configurable": {"thread_id": f"architect-{spec_id}"}},
    )
    return result.get("analysis", {})
