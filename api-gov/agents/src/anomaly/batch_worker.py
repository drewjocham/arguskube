"""Batch worker — reads JSONL drift candidate files, groups by spec_id,
calls LLM once per batch, persists results.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import logging
import os
import time
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from src.anomaly.metadata_filter import MetadataFilter
from src.config import config
from src.database import db
from src.redis_client import redis_client

logger = logging.getLogger(__name__)

BATCH_PROMPT = """You are a Sentinel reviewing a batch of API drift candidates for spec {spec_id}.
Review each candidate and decide if it's a real drift or a false positive.

Return a JSON array of confirmed drifts. Skip false positives and noise.
For each confirmed drift, include: severity, category, field, issue, suggestion, score.

Batch of {count} candidates:
{batch}
"""


async def ensure_batch_dir() -> None:
    Path(config.llm_batch_dir).mkdir(parents=True, exist_ok=True)


def _batch_path(spec_id: str) -> str:
    return os.path.join(config.llm_batch_dir, f"{spec_id}.jsonl")


async def write_candidate(spec_id: str, candidate: dict) -> None:
    """Append a single drift candidate to the spec's JSONL batch file."""
    await ensure_batch_dir()
    meta = MetadataFilter.make_metadata(candidate)
    line = json.dumps({"metadata": meta, "candidate": candidate, "ts": time.time()})
    path = _batch_path(spec_id)
    with open(path, "a") as f:
        f.write(line + "\n")


async def read_batch(spec_id: str) -> list[dict]:
    """Read all unprocessed candidates for a spec. Returns empty list if none.
    After reading, truncates the file so each batch is processed at most once.
    """
    path = _batch_path(spec_id)
    if not os.path.exists(path):
        return []

    candidates = []
    with open(path, "r") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
                candidates.append(entry["candidate"])
            except (json.JSONDecodeError, KeyError):
                continue

    # Truncate the file — these candidates are now "in flight"
    with open(path, "w") as f:
        f.write("")

    if not candidates:
        return []

    # Deduplicate via metadata filter before LLM
    filtered = await MetadataFilter.filter_new(spec_id, candidates)
    return filtered


async def process_batch(spec_id: str) -> list[dict]:
    """Process one batch for a spec: read candidates, call LLM, persist results."""
    candidates = await read_batch(spec_id)
    if not candidates:
        return []

    llm = config.create_llm(temperature=0.0)
    batch_str = json.dumps(candidates, indent=2, default=str)

    # Truncate to model's context limit (leave room for instructions + output)
    max_input = 40000  # ~40K chars = ~10K tokens, well within 128K context
    if len(batch_str) > max_input:
        batch_str = batch_str[:max_input] + "\n  ... truncated"

    prompt = BATCH_PROMPT.format(
        spec_id=spec_id,
        count=len(candidates),
        batch=batch_str,
    )

    # Semantic cache check
    cache_key = f"llm:batch:cache:{hashlib.sha256(prompt.encode()).hexdigest()[:16]}"
    cached = await redis_client._pool.get(cache_key) if redis_client._pool else None
    if cached:
        try:
            reports = json.loads(cached)
            logger.info("batch cache hit for spec %s (%d reports)", spec_id, len(reports))
            return await _persist_reports(spec_id, candidates, reports)
        except json.JSONDecodeError:
            pass

    response = await llm.ainvoke(prompt)
    content = response.content if hasattr(response, "content") else str(response)

    # Cache result for 1 hour
    if redis_client._pool:
        await redis_client._pool.setex(cache_key, 3600, content)

    try:
        reports = json.loads(content.strip().removeprefix("```json").removeprefix("```").removesuffix("```"))
    except (json.JSONDecodeError, AttributeError):
        logger.warning("batch LLM parse failed for spec %s", spec_id)
        return []

    return await _persist_reports(spec_id, candidates, reports)


async def _persist_reports(spec_id: str, candidates: list[dict], reports: list[dict]) -> list[dict]:
    """Mark candidates as seen and persist confirmed reports."""
    for r in reports:
        r["spec_id"] = spec_id
        r["source"] = "batch_llm"
        r["score"] = r.get("score", 0.5)

    # Mark all assessed candidates as seen
    for c in candidates:
        await MetadataFilter.mark_seen(
            spec_id,
            c.get("method", "GET"),
            c.get("path", "/"),
            c.get("field", ""),
            c.get("category", "unknown"),
            result="assessed",
        )

    # Persist significant drifts
    significant = [r for r in reports if r.get("score", 1.0) < config.drift_threshold]
    if significant:
        await db.save_drift_reports(significant)
        logger.info("batch: persisted %d/%d drifts for spec %s", len(significant), len(reports), spec_id)

    return significant


async def run_batch_cycle() -> None:
    """Run one batch cycle: find all specs with pending candidates, process them."""
    await ensure_batch_dir()
    batch_dir = Path(config.llm_batch_dir)

    spec_files = list(batch_dir.glob("*.jsonl"))
    if not spec_files:
        return

    for fpath in spec_files:
        spec_id = fpath.stem  # filename without .jsonl = spec_id
        try:
            await process_batch(spec_id)
        except Exception as e:
            logger.exception("batch processing failed for spec %s: %s", spec_id, e)

    logger.info("batch cycle complete: %d specs processed", len(spec_files))


async def batch_loop() -> None:
    """Background loop: runs batch cycle every `llm_batch_window_sec` seconds."""
    while True:
        try:
            await run_batch_cycle()
        except Exception as e:
            logger.exception("batch cycle failed: %s", e)
        await asyncio.sleep(config.llm_batch_window_sec)
