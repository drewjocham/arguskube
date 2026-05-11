"""Redis-backed counters for anomaly detection — no raw event storage.

Uses HyperLogLog for cardinality estimation and INCR + EXPIRE for
sliding-window rate counters. Memory per spec: ~16KB regardless of traffic volume.
"""

from __future__ import annotations

import math
import time
from typing import Any

from src.redis_client import redis_client

# Redis key prefixes
HLL_KEY = "anomaly:hll:{spec_id}:{metric}"          # HyperLogLog for cardinality
RATE_KEY = "anomaly:rate:{spec_id}:{metric}:{window}" # INCR counter per window
STATUS_KEY = "anomaly:status:{spec_id}:{code}"        # Status code counter
FIELD_KEY = "anomaly:field:{spec_id}:{endpoint}"      # Field presence Bloom-like

# Window sizes in seconds
WINDOWS = [60, 300, 3600]  # 1min, 5min, 1hr
HLL_TTL = 86400  # 24hr


class AnomalyCounters:
    """Zero-storage counters for API anomaly detection at scale."""

    @staticmethod
    async def record_event(spec_id: str, method: str, path: str, status: int, latency_ms: float) -> None:
        """Record a traffic event across all counter dimensions."""
        pipe = redis_client._pool.pipeline() if redis_client._pool else None
        if not pipe:
            return

        now = int(time.time())

        # 1. HyperLogLog — unique endpoint cardinality per spec
        hll_key = HLL_KEY.format(spec_id=spec_id, metric="endpoints")
        pipe.pfadd(hll_key, f"{method}:{path}")
        pipe.expire(hll_key, HLL_TTL)

        # 2. Status code counter (sliding 1min window)
        for window in WINDOWS:
            bucket = now - (now % window)
            rate_key = RATE_KEY.format(spec_id=spec_id, metric="traffic", window=window)
            pipe.incr(f"{rate_key}:{bucket}")
            pipe.expire(f"{rate_key}:{bucket}", window * 2)

        # 3. Per-status code counts
        status_key = STATUS_KEY.format(spec_id=spec_id, code=status)
        pipe.incr(status_key)
        pipe.expire(status_key, HLL_TTL)

        # 4. Latency bucket — running stats stored as sorted set
        latency_bucket = int(latency_ms / 50) * 50  # 50ms buckets
        pipe.zincrby(f"anomaly:latency:{spec_id}", 1, latency_bucket)
        pipe.expire(f"anomaly:latency:{spec_id}", HLL_TTL)

        await pipe.execute()

    @staticmethod
    async def get_traffic_rate(spec_id: str, window: int = 300) -> float:
        """Average requests/sec over the given window."""
        now = int(time.time())
        bucket = now - (now % window)
        rate_key = RATE_KEY.format(spec_id=spec_id, metric="traffic", window=window)
        if not redis_client._pool:
            return 0.0
        count = await redis_client._pool.get(f"{rate_key}:{bucket}")
        return int(count or 0) / window

    @staticmethod
    async def get_endpoint_cardinality(spec_id: str) -> int:
        """Estimated unique endpoints seen."""
        key = HLL_KEY.format(spec_id=spec_id, metric="endpoints")
        if not redis_client._pool:
            return 0
        return await redis_client._pool.pfcount(key)

    @staticmethod
    async def get_status_counts(spec_id: str) -> dict[int, int]:
        """Current status code distribution."""
        if not redis_client._pool:
            return {}
        keys = await redis_client._pool.keys(STATUS_KEY.format(spec_id=spec_id, code="*"))
        result = {}
        for k in keys:
            code = int(k.split(":")[-1])
            val = await redis_client._pool.get(k)
            if val:
                result[code] = int(val)
        return result

    @staticmethod
    async def get_latency_percentiles(spec_id: str) -> dict[str, float]:
        """Estimated p50/p95/p99 latency from histogram."""
        key = f"anomaly:latency:{spec_id}"
        if not redis_client._pool:
            return {}
        total = await redis_client._pool.zcard(key)
        if total == 0:
            return {"p50": 0, "p95": 0, "p99": 0}

        data = await redis_client._pool.zrange(key, 0, -1, withscores=True)
        buckets = sorted([(int(b), int(c)) for b, c in data])
        cumulative = 0
        result = {}
        for bucket, count in buckets:
            cumulative += count
            pct = cumulative / total
            if p50 is None and pct >= 0.5:
                result["p50"] = float(bucket)
            if p95 is None and pct >= 0.95:
                result["p95"] = float(bucket)
            if p99 is None and pct >= 0.99:
                result["p99"] = float(bucket)
        return result
