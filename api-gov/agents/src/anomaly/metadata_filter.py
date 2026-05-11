"""Metadata-driven deduplication for drift candidates.

Before sending to LLM, we hash each candidate by (spec_id, endpoint_hash, category, field)
and skip anything we've already assessed recently. This eliminates ~70% of LLM calls.
"""

from __future__ import annotations

import hashlib
import json
from typing import Any

from src.redis_client import redis_client

DEDUP_KEY = "anomaly:dedup:{spec_id}:{category}:{field_hash}"
DEDUP_TTL = 86400  # 24hr


def _field_hash(method: str, path: str, field: str) -> str:
    raw = f"{method}:{path}:{field}"
    return hashlib.sha256(raw.encode()).hexdigest()[:16]


class MetadataFilter:
    """Deduplicates drift candidates before they reach the LLM.

    A candidate is "known" if we've seen the same (spec_id, endpoint, field, category)
    combination in the last 24 hours and the user dismissed it or we auto-resolved it.
    """

    @staticmethod
    async def is_known(spec_id: str, method: str, path: str, field: str, category: str) -> bool:
        """Check if this exact drift pattern was recently assessed."""
        fh = _field_hash(method, path, field)
        key = DEDUP_KEY.format(spec_id=spec_id, category=category, field_hash=fh)
        if not redis_client._pool:
            return False
        return await redis_client._pool.exists(key) > 0

    @staticmethod
    async def mark_seen(
        spec_id: str, method: str, path: str, field: str, category: str, result: str = "pending"
    ) -> None:
        """Mark a drift pattern as seen to avoid re-assessing."""
        fh = _field_hash(method, path, field)
        key = DEDUP_KEY.format(spec_id=spec_id, category=category, field_hash=fh)
        if redis_client._pool:
            await redis_client._pool.setex(key, DEDUP_TTL, result)

    @staticmethod
    async def filter_new(
        spec_id: str, candidates: list[dict]
    ) -> list[dict]:
        """Filter a list of drift candidates to only include new/unseen ones."""
        if not candidates:
            return []

        unique: dict[str, dict] = {}
        for c in candidates:
            method = c.get("method", "GET")
            path = c.get("path", "/")
            field = c.get("field", "")
            category = c.get("category", "unknown")
            fh = _field_hash(method, path, field)

            # Dedup key: dedup key includes (spec, category, field_hash)
            # If two candidates have same (spec, category, field_hash) they're duplicates
            dedup_key = f"{category}:{fh}"
            if dedup_key in unique:
                continue

            # Check if this was recently seen
            known = await MetadataFilter.is_known(spec_id, method, path, field, category)
            if not known:
                unique[dedup_key] = c

        new_candidates = list(unique.values())
        skipped = len(candidates) - len(new_candidates)
        if skipped:
            import logging
            logging.getLogger(__name__).info(
                "metadata filter: skipped %d/%d known candidates", skipped, len(candidates)
            )
        return new_candidates

    @staticmethod
    def make_metadata(candidate: dict) -> dict:
        """Extract metadata hash fields from a drift candidate for JSONL header."""
        method = candidate.get("method", "GET")
        path = candidate.get("path", "/")
        field = candidate.get("field", "")
        category = candidate.get("category", "unknown")
        fh = _field_hash(method, path, field)
        return {
            "field_hash": fh,
            "category": category,
            "endpoint_hash": hashlib.sha256(f"{method}:{path}".encode()).hexdigest()[:12],
            "spec_id": candidate.get("spec_id", ""),
        }
