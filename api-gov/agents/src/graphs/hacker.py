from __future__ import annotations

import json
import logging

from langgraph.graph import END, StateGraph
from langgraph.checkpoint.memory import MemorySaver
from langgraph.checkpoint.redis import RedisSaver

from src.config import config
from src.graphs.state import AgentState

logger = logging.getLogger(__name__)

HACKER_PROMPT = """You are a Hacker Agent generating edge-case test payloads.
For the given endpoint schema, generate test cases that probe for boundary values, type violations,
missing required fields, auth bypass, and injection attempts.

Return a JSON array:
[{"name": str, "method": str, "path": str, "headers": dict, "body": object|null, "expected_status": int, "description": str}]

Endpoint:
{m}')
```


async def load_endpoints(state: AgentState) -> dict:
    from src.database import db
    endpoints = await db.fetch_endpoints(state.spec_id)
    return {"endpoints": endpoints}


async def select_target(state: AgentState) -> dict:
    target_id = state.messages[0].get("endpoint_id", "") if state.messages else ""
    if target_id:
        target = [ep for ep in state.endpoints if ep.get("id") == target_id]
    else:
        target = state.endpoints[:3]
    return {"endpoints": target}


async def llm_generate(state: AgentState) -> dict:
    if not state.endpoints:
        return {"test_cases": []}

    llm = config.create_llm(temperature=0.2)
    count = state.messages[0].get("count", 5) if state.messages else 5
    all_tests = []

    for ep in state.endpoints[:3]:
        ep_str = json.dumps(ep, indent=2, default=str)[:4000]
        prompt = HACKER_PROMPT.format(m=ep_str, count=count)
        response = await llm.ainvoke(prompt)

        try:
            content = response.content if hasattr(response, "content") else str(response)
            tests = json.loads(content.strip().removeprefix("```json").removeprefix("```").removesuffix("```"))
            for t in tests:
                t["method"] = t.get("method", ep.get("method", "GET"))
                t["path"] = t.get("path", ep.get("path", "/"))
            all_tests.extend(tests)
        except (json.JSONDecodeError, AttributeError):
            logger.warning("LLM parse failed for %s %s", ep.get("method"), ep.get("path"))

    return {"test_cases": all_tests}


async def persist_tests(state: AgentState) -> dict:
    if state.test_cases:
        from src.database import db
        await db.save_generated_tests(state.spec_id, state.test_cases)
        logger.info("persisted %d test cases", len(state.test_cases))
    return {}


def build_hacker_graph(redis_url: str | None = None) -> StateGraph:
    workflow = StateGraph(AgentState)
    workflow.add_node("load_endpoints", load_endpoints)
    workflow.add_node("select_target", select_target)
    workflow.add_node("llm_generate", llm_generate)
    workflow.add_node("persist_tests", persist_tests)
    workflow.set_entry_point("load_endpoints")
    workflow.add_edge("load_endpoints", "select_target")
    workflow.add_edge("select_target", "llm_generate")
    workflow.add_edge("llm_generate", "persist_tests")
    workflow.add_edge("persist_tests", END)

    if redis_url:
        checkpointer = RedisSaver(redis_url)
    else:
        checkpointer = MemorySaver()
    return workflow.compile(checkpointer=checkpointer)


hacker_graph = build_hacker_graph()


async def generate(spec_id: str, endpoint_id: str | None = None, count: int = 5) -> list[dict]:
    initial = AgentState(spec_id=spec_id, messages=[{"endpoint_id": endpoint_id or "", "count": count}])
    result = await hacker_graph.ainvoke(
        initial,
        config={"configurable": {"thread_id": f"hacker-{spec_id}-{endpoint_id or 'all'}"}},
    )
    return result.get("test_cases", [])
