# ADR-003: LLM Provider Selection

**Status:** Accepted  
**Date:** 2026-05-11  

## Context
The agent system uses LLMs for three distinct tasks: spec analysis (architect), drift verification (sentinel layer 2), test generation (hacker), and fix suggestions (healer). Each has different latency, cost, and quality requirements.

## Decision

**Primary Provider: SiliconFlow (DeepSeek-V3-0324)**  
**Fallback Provider: OpenAI (GPT-4o-mini)** — for rate-limit resilience

**Why SiliconFlow:**
- OpenAI-compatible API — zero code changes to switch
- DeepSeek-V3: ~$0.50/M input tokens vs GPT-4o's ~$2.50/M (80% cost reduction)
- Long context window (128K vs 8K) for full spec analysis
- Chinese-hosted but API-compatible — no vendor lock-in

**Per-Agent Model Assignment:**

| Agent | Model | Temperature | Max Tokens | Rationale |
|-------|-------|-------------|------------|-----------|
| Architect | DeepSeek-V3 | 0.0 | 1024 | Deterministic analysis, structured JSON output |
| Sentinel | DeepSeek-V3 | 0.0 | 512 | Binary classification (drift vs not) |
| Hacker | DeepSeek-V3 | 0.2 | 2048 | Creative test generation needs some randomness |
| Healer | DeepSeek-V3 | 0.0 | 1024 | Precise fix suggestions |
| Orchestrator | DeepSeek-V3 | 0.1 | 512 | Coordination + spot checks |

**Dual-LLM Asymmetry** (noted in review):
- Go backend defaults to Gemini 1.5 Pro (`config.go:35`)
- Python agents use DeepSeek-V3 via SiliconFlow (`config.py:11`)
- These are independent — Go uses LLM for analysis orchestration (future), agents use LLM for drift tasks
- No conflict, but worth unifying if Go ever directly calls LLMs

## Cost Model

| Tier | Filter | % of events | Calls/hr at 50k | Cost/hr |
|------|--------|-------------|-----------------|---------|
| Architect | Per-spec upload | — | 50k (one-time) | Negligible |
| Sentinel L2 | Ambiguous drifts only | ~5% | 250k peak | $81 peak |
| Hacker | On-demand | — | Negligible | — |
| Healer | Per-drift | — | Event-driven | — |

Peak cost: ~$59k/mo (sustained 24/7). Mitigated by:
1. Semantic cache (70% cache hit → $18k/mo)
2. Per-spec rate limit (5 calls/hr hard cap)
3. Structural checks reduce L2 reach by 95%
