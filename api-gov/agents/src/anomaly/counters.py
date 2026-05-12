from __future__ import annotations

import math
import time
from typing import Any

from src.redis_client import redis_client

# Redis key prefixes
HLL_KEY = "anomaly:hll:{spec_id}:{metric}"            
RATE_KEY = "anomaly:rate:{spec_id}:{metric}:{window}" 
STATUS_KEY = "anomaly:status:{spec_id}"               # Changed to Hash key
LATENCY_KEY = "anomaly:latency:{spec_id}"             # Sorted set key

# Window sizes in seconds
WINDOWS = [60, 300, 3600]  # 1min, 5min, 1hr
HLL_TTL = 86400  # 24hr


class AnomalyCounters:
    """Zero-storage counters for API anomaly detection at scale."""

    @staticmethod
    async def record_event(spec_id: str, method: str, path: str, status: int, latency_ms: float) -> None:
        """Record a traffic event across all counter dimensions."""
        if not redis_client._pool:
            return
            
        pipe = redis_client._pool.pipeline()
        now = int(time.time())

        # HyperLogLog — unique endpoint cardinality
        hll_key = HLL_KEY.format(spec_id=spec_id, metric="endpoints")
        pipe.pfadd(hll_key, f"{method}:{path}")
        pipe.expire(hll_key, HLL_TTL)

        # Rate counters (bucketing)
        for window in WINDOWS:
            bucket = now - (now % window)
            rate_key = RATE_KEY.format(spec_id=spec_id, metric="traffic", window=window)
            pipe.incr(f"{rate_key}:{bucket}")
            pipe.expire(f"{rate_key}:{bucket}", window * 2)

        # Status code counts (Using HASH for O(1) lookups and avoiding KEYS)
        status_key = STATUS_KEY.format(spec_id=spec_id)
        pipe.hincrby(status_key, str(status), 1)
        pipe.expire(status_key, HLL_TTL)

        # Latency bucket
        latency_bucket = int(latency_ms / 50) * 50  # 50ms buckets
        latency_key = LATENCY_KEY.format(spec_id=spec_id)
        pipe.zincrby(latency_key, 1, latency_bucket)
        pipe.expire(latency_key, HLL_TTL) 

        await pipe.execute()

    @staticmethod
    async def get_traffic_rate(spec_id: str, window: int = 300) -> float:
        """Average requests/sec for the current window based on elapsed time."""
        if not redis_client._pool:
            return 0.0
            
        now = int(time.time())
        bucket_start = now - (now % window)
        elapsed = max(1, now - bucket_start) # Prevent division by zero
        
        rate_key = RATE_KEY.format(spec_id=spec_id, metric="traffic", window=window)
        count = await redis_client._pool.get(f"{rate_key}:{bucket_start}")
        
        return int(count or 0) / elapsed

    @staticmethod
    async def get_endpoint_cardinality(spec_id: str) -> int:
        """Estimated unique endpoints seen."""
        if not redis_client._pool:
            return 0
            
        key = HLL_KEY.format(spec_id=spec_id, metric="endpoints")
        return await redis_client._pool.pfcount(key)

    @staticmethod
    async def get_status_counts(spec_id: str) -> dict[int, int]:
        """Current status code distribution using HGETALL."""
        if not redis_client._pool:
            return {}
            
        status_key = STATUS_KEY.format(spec_id=spec_id)
        raw_data = await redis_client._pool.hgetall(status_key)
        
        # Redis returns bytes or strings depending on the driver configuration
        return {int(k): int(v) for k, v in raw_data.items()}

    @staticmethod
    async def get_latency_percentiles(spec_id: str) -> dict[str, float]:
        """Estimated p50/p95/p99 latency from histogram."""
        if not redis_client._pool:
            return {"p50": 0.0, "p95": 0.0, "p99": 0.0}
            
        key = LATENCY_KEY.format(spec_id=spec_id)
        data = await redis_client._pool.zrange(key, 0, -1, withscores=True)
        
        if not data:
            return {"p50": 0.0, "p95": 0.0, "p99": 0.0}

        # Calculate true total by summing the scores (counts)
        total_events = sum(score for _, score in data)
        if total_events == 0:
             return {"p50": 0.0, "p95": 0.0, "p99": 0.0}

        buckets = sorted([(float(b), int(c)) for b, c in data])
        cumulative = 0
        
        # Initialize tracking variables to prevent NameError
        p50 = p95 = p99 = None 
        result = {}
        
        for bucket, count in buckets:
            cumulative += count
            pct = cumulative / total_events
            
            if p50 is None and pct >= 0.50:
                p50 = result["p50"] = bucket
            if p95 is None and pct >= 0.95:
                p95 = result["p95"] = bucket
            if p99 is None and pct >= 0.99:
                p99 = result["p99"] = bucket

        # Fallback in case rounding prevents exact matching on the highest bounds
        result.setdefault("p50", buckets[-1][0])
        result.setdefault("p95", buckets[-1][0])
        result.setdefault("p99", buckets[-1][0])
        
        return result