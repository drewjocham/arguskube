# 2026-04-28
payments-api heap config frozen at 512Mi pending batch-size review — blocked
owner: @sre-team · ticket: SRE-412
Note: v2.4.0 stable with batch size 3,200. Any increase requires memory limit review.

# 2026-04-25
order-processor HPA min replicas set to 2 (was 3) to reduce idle cost.
cpu request: 200m, limit: 500m.
HPA target: 70% CPU.
owner: @platform-team · ticket: PLAT-891

# 2026-04-22
prometheus retention reduced from 30d to 15d on worker-node-09 due to disk constraints.
WAL compaction scheduled nightly at 03:00 UTC.
PVC size: 22GB — expansion request pending approval.
owner: @observability-team · ticket: OBS-203

# 2026-04-18
redis-cluster upgraded to 7.2.4. Connection pooling parameters unchanged.
payments-api and order-processor both depend on redis for session cache.
No issues observed post-upgrade.

# 2026-04-15
nginx-ingress rate limiting enabled: 100 req/s per client IP.
Affects payments flow during flash sales — monitor order-processor queue depth.
owner: @sre-team · ticket: SRE-398

# 2026-04-10
metrics-server resource limits set to 100m CPU / 256Mi memory.
Running on worker-node-09 (same node as prometheus).
If node pressure occurs, metrics-server may be evicted — this would break HPA for all namespaces.
owner: @platform-team · ticket: PLAT-867
