# ADR-001: LangGraph for Agent Orchestration

**Status:** Accepted  
**Date:** 2026-05-11  
**Author:** Architectural Review  

## Context

The api-gov platform requires a multi-agent system for API drift detection, fuzz testing, and auto-healing. Agents need state management, branching, checkpointing, and human-in-the-loop support.

## Considered Alternatives

| Option | Pros | Cons |
|--------|------|------|
| **LangGraph** | Native DAG/cyclic graphs, LangChain integration, built-in checkpointing, async support | Python-only, relatively new ecosystem |
| **Temporal** | Battle-tested, long-running workflow support, language-agnostic | Heavy infra (Temporal Server + DB), no native LLM integration |
| **AWS Step Functions** | Serverless, no infra to manage | Vendor lock-in, JSON state machine DSL, no LLM primitives |
| **CrewAI** | Simpler API, role-based agents | Less flexible state model, no checkpointing, smaller ecosystem |
| **Plain asyncio** | No dependencies, full control | Manual state management, no built-in DAG execution, no checkpointing |

## Decision

**LangGraph** with `MemorySaver` for development, `RedisSaver` for production.

**Rationale:**
- State graph model maps directly to drift detection workflow (observe → detect → score → persist)
- LangChain integration gives access to SiliconFlow via `ChatOpenAI` without custom adapters
- Built-in checkpointing per thread (`thread_id = spec_id + action`) enables per-tenant isolation
- Cyclic graphs support the orchestrator's refine-loop pattern
- Conditional edges handle tiered detection (skip LLM tier if structural check passes)

## Tradeoffs
- **In-memory checkpointing only in dev**: `MemorySaver` loses state on restart. Production must use `RedisSaver` (already have Redis in the stack).
- **Python-only agents**: Go backend calls agents via HTTP. Consider gRPC for lower latency at >50k RPS.

## Consequences
- All LangGraph checkers in `src/graphs/*.py` must accept a configurable `checkpointer` parameter
- Production deployment requires Redis (already in docker-compose)
- Agent crashes lose in-flight work until `RedisSaver` is wired (P0)
