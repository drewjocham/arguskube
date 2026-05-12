# ADR-002: Redis Counter Design — Zero Raw Event Storage

**Status:** Accepted  
**Date:** 2026-05-11  

## Context

At 50k specs × variable traffic, storing raw traffic events for anomaly detection is prohibitively expensive. Each event at ~1KB would generate 50GB+/day. We need to detect drift without storing individual requests.

## Decision

**No raw event storage.** Replace with three tiered Redis structures:

| Structure | Memory/Spec | What it tracks | Loss on Restart |
|-----------|-------------|----------------|-----------------|
| HyperLogLog (PFADD) | ~12KB | Unique endpoint cardinality | Rebuild from PG endpoints |
| INCR + EXPIRE windows | ~100B × 3 windows | Request rate per time bucket | Accept reset |
| Running stats (Welford) | 4 floats/spec | Mean, variance, M2, n | Accept warm-up |
| Field presence hashes | O(fields) × spec | Field occurrence ratios | Rebuild from traffic replay |
| Latency histogram (ZSET) | O(buckets) × spec | p50/p95/p99 estimates | Accept reset |

**Foreclosed queries** (we cannot answer these without raw storage):
- "Show me the exact request that triggered the alert" — need traffic mirror (sidecar)
- "Replay all events from last hour" — need separate audit log in PG
- "What was the exact response body for spec X at time T" — need user to enable verbose mode

## Alternatives Considered
- **TimescaleDB**: Full SQL, continuous aggregates, but adds PG extension complexity. No advantage for real-time per-event checks.
- **ClickHouse**: Columnar, excellent for analytics, but separate infra to manage.
- **Raw event storage in S3**: Durable but 10min+ query latency.

## Consequences
- Redis memory ~16KB/spec = ~800MB at 50k (negligible)
- Trafficevent replay requires separate traffic mirror sidecar
- z-score thresholds adapt over ~10 observations
- No ability to retroactively change drift detection logic (must forward-process)
