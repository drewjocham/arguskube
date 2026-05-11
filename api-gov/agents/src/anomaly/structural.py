"""Field-level structural tracking for API drift detection.

Uses Redis hashes with field presence ratios — no raw payload storage.
Each endpoint stores field names as hash keys, values are:
  count_seen:total_requests (ratio of requests containing this field).

Memory per endpoint: O(fields) — typically < 100 bytes.
"""

from __future__ import annotations

import hashlib
from typing import Any

from src.redis_client import redis_client

FIELD_KEY = "anomaly:fields:{spec_id}:{method}:{path_hash}"
FIELD_TTL = 86400 * 7  # 7 days


def _path_hash(method: str, path: str) -> str:
    return hashlib.md5(f"{method}:{path}".encode()).hexdigest()[:12]


class FieldTracker:
    """Tracks field presence ratios per endpoint. Detects missing or undocumented fields."""

    @staticmethod
    async def record_fields(spec_id: str, method: str, path: str, fields: list[str]) -> None:
        """Record observed fields for an endpoint. Updates presence ratios."""
        key = FIELD_KEY.format(spec_id=spec_id, method=method, path_hash=_path_hash(method, path))
        if not redis_client._pool:
            return

        pipe = redis_client._pool.pipeline()
        for field in fields:
            pipe.hincrby(key, f"{field}:seen", 1)
        pipe.hincrby(key, "_total", 1)
        pipe.expire(key, FIELD_TTL)
        await pipe.execute()

    @staticmethod
    async def get_field_profile(spec_id: str, method: str, path: str) -> dict[str, float]:
        """Get field presence ratios for an endpoint.
        Returns {field_name: presence_ratio (0.0-1.0), ...}.
        """
        key = FIELD_KEY.format(spec_id=spec_id, method=method, path_hash=_path_hash(method, path))
        if not redis_client._pool:
            return {}

        raw = await redis_client._pool.hgetall(key)
        if not raw:
            return {}

        total = int(raw.pop(b"_total", raw.pop("_total", 0)))
        if total == 0:
            return {}

        result = {}
        for k, v in raw.items():
            field_name = k.decode() if isinstance(k, bytes) else str(k)
            count = int(v)
            if field_name.endswith(":seen"):
                name = field_name[:-5]
                result[name] = round(count / total, 4)
        return result

    @staticmethod
    async def detect_new_fields(
        spec_id: str, method: str, path: str, observed_fields: list[str]
    ) -> list[str]:
        """Return fields that have never been seen before for this endpoint."""
        key = FIELD_KEY.format(spec_id=spec_id, method=method, path_hash=_path_hash(method, path))
        if not redis_client._pool:
            return []

        result = []
        for field in observed_fields:
            seen = await redis_client._pool.hexists(key, f"{field}:seen")
            if not seen:
                result.append(field)
        return result

    @staticmethod
    async def get_all_profiles(spec_id: str) -> dict[str, dict[str, float]]:
        """Get field profiles for all endpoints under a spec."""
        if not redis_client._pool:
            return {}

        pattern = FIELD_KEY.format(spec_id=spec_id, method="*", path_hash="*")
        keys = await redis_client._pool.keys(pattern)
        profiles = {}
        for key in keys:
            k = key.decode() if isinstance(key, bytes) else key
            parts = k.split(":")
            method = parts[-2]
            raw = await redis_client._pool.hgetall(key)
            total = int(raw.pop(b"_total", raw.pop("_total", 0)))
            if total == 0:
                continue
            fields = {}
            for field_key, v in raw.items():
                fk = field_key.decode() if isinstance(field_key, bytes) else str(field_key)
                if fk.endswith(":seen"):
                    fields[fk[:-5]] = round(int(v) / total, 4)
            profiles[f"{method}:{parts[-1]}"] = fields
        return profiles
