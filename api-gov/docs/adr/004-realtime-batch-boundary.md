# ADR-004: Real-time vs Batch Processing Boundary

**Status:** Accepted  
**Date:** 2026-05-11  

## Decision
Split anomaly detection into three processing tiers with explicit latency and consistency boundaries.

## Tier Definitions

| Tier | Latency | Trigger | Actions | Storage |
|------|---------|---------|---------|---------|
| **L0 — Counters** | <1ms | Every event | PFADD, INCR, ZINCRBY | Redis |
| **L1 — Statistical** | <10ms | Every event | Z-score compute, field ratio update, threshold check | Redis + PG (drift reports) |
| **L2 — LLM** | <5s | L1 flags ambiguous | LLM verification | PG (drift reports) |
| **Batch — ML** | 5min | Cron | DuckDB flush → Anomstack train/score → webhook | DuckDB + Anomstack |

## Split Criteria
- **Real-time (L0-L2):** Anything that affects the immediate user experience (drift alert, test failure)
- **Batch (Anomstack):** Trending/pattern analysis that benefits from historical context (latency trends, seasonal patterns, model drift over weeks)

## Consistency
- L0-L2 are eventually consistent: a traffic spike may produce a false positive that's corrected when Anomstack batch runs
- Batch results feed back into L1 threshold tuning (if Anomstack finds a new baseline, update z-score window)
- No strong consistency requirements across tiers — each tier operates independently

## Foreclosed
- Raw event replay is not supported at any tier. Events flow through L0 counters and are discarded.
- Cross-tier joins are not possible (no shared event ID)
