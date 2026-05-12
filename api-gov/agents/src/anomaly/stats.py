"""Online running statistics — Welford's algorithm for O(1) memory anomaly detection.

Stores only 4 floats per spec: n, mean, M2 (used for variance), last_seen.
No event history stored.
"""

from __future__ import annotations

import math
import time
from typing import Any

from src.redis_client import redis_client

STATS_KEY = "anomaly:stats:{spec_id}:{metric}"


class RunningStats:
    """Running mean/stddev using Welford's online algorithm.

    Memory per metric per spec: 4 float64 values stored in Redis hash.
    """

    @staticmethod
    def _key(spec_id: str, metric: str) -> str:
        return STATS_KEY.format(spec_id=spec_id, metric=metric)

    @staticmethod
    async def update(spec_id: str, metric: str, value: float) -> dict[str, float]:
        """Update running stats with a new observation. Returns current stats."""
        key = RunningStats._key(spec_id, metric)

        if not redis_client._pool:
            return {"n": 0, "mean": 0.0, "stddev": 0.0}

        async with redis_client._pool.pipeline() as pipe:
            pipe.hgetall(key)
            pipe.time()
            results = await pipe.execute()

        raw = results[0] or {}
        server_time = float(results[1][0]) + float(results[1][1]) / 1_000_000

        n = int(raw.get(b"n", raw.get("n", 0)))
        mean = float(raw.get(b"mean", raw.get("mean", 0.0)))
        m2 = float(raw.get(b"m2", raw.get("m2", 0.0)))

        # Welford's online
        n += 1
        delta = value - mean
        mean += delta / n
        delta2 = value - mean
        m2 += delta * delta2
        stddev = math.sqrt(m2 / n) if n > 1 else 0.0

        await redis_client._pool.hset(key, mapping={
            "n": str(n),
            "mean": str(mean),
            "m2": str(m2),
            "last_seen": str(server_time),
        })
        await redis_client._pool.expire(key, 86400 * 7)

        return {"n": n, "mean": round(mean, 4), "stddev": round(stddev, 4)}

    @staticmethod
    async def z_score(spec_id: str, metric: str, value: float) -> float:
        """Compute how many stddevs the value is from the mean. Returns z-score."""
        key = RunningStats._key(spec_id, metric)
        if not redis_client._pool:
            return 0.0

        raw = await redis_client._pool.hgetall(key)
        if not raw:
            return 0.0

        n = int(raw.get(b"n", raw.get("n", 0)))
        mean = float(raw.get(b"mean", raw.get("mean", 0.0)))
        m2 = float(raw.get(b"m2", raw.get("m2", 0)))
        stddev = math.sqrt(m2 / n) if n > 1 else 0.0

        if stddev == 0:
            return 0.0
        return round((value - mean) / stddev, 4)

    @staticmethod
    async def get_stats(spec_id: str, metric: str) -> dict[str, float]:
        """Retrieve current stats without updating."""
        key = RunningStats._key(spec_id, metric)
        if not redis_client._pool:
            return {"n": 0, "mean": 0.0, "stddev": 0.0, "z_score_threshold": 0.0}

        raw = await redis_client._pool.hgetall(key)
        if not raw:
            return {"n": 0, "mean": 0.0, "stddev": 0.0, "z_score_threshold": 0.0}

        n = int(raw.get(b"n", raw.get("n", 0)))
        mean = float(raw.get(b"mean", raw.get("mean", 0.0)))
        m2 = float(raw.get(b"m2", raw.get("m2", 0)))
        stddev = math.sqrt(m2 / n) if n > 1 else 0.0

        # Adaptive threshold: narrow if we have lots of data
        threshold = 4.0 if n < 100 else 3.5 if n < 1000 else 3.0
        return {
            "n": n,
            "mean": round(mean, 4),
            "stddev": round(stddev, 4),
            "z_score_threshold": threshold,
        }
