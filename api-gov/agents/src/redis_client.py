from __future__ import annotations

import json
import logging
import time
from typing import Any

import redis.asyncio as aioredis

from src.config import config

logger = logging.getLogger(__name__)

TRAFFIC_KEY_PREFIX = "traffic:"
TRAFFIC_MAX_EVENTS = 500
TRAFFIC_TTL = 3600
PROFILE_KEY_PREFIX = "profile:"
CHECKPOINT_KEY_PREFIX = "checkpoint:"


class RedisClient:
    def __init__(self) -> None:
        self._pool: aioredis.Redis | None = None

    async def connect(self) -> None:
        self._pool = aioredis.from_url(
            config.redis_url,
            decode_responses=True,
            socket_timeout=5,
            socket_connect_timeout=5,
        )
        await self._pool.ping()
        logger.info("redis connected")

    async def close(self) -> None:
        if self._pool:
            await self._pool.aclose()

    async def push_traffic(self, spec_id: str, event: dict) -> int:
        if not self._pool:
            return 0
        key = f"{TRAFFIC_KEY_PREFIX}{spec_id}"
        pipe = self._pool.pipeline()
        pipe.lpush(key, json.dumps(event, default=str))
        pipe.ltrim(key, 0, TRAFFIC_MAX_EVENTS - 1)
        pipe.expire(key, TRAFFIC_TTL)
        pipe.llen(key)
        results = await pipe.execute()
        return results[-1]

    async def pop_traffic_batch(self, spec_id: str, count: int = 50) -> list[dict]:
        if not self._pool:
            return []
        key = f"{TRAFFIC_KEY_PREFIX}{spec_id}"
        raw = await self._pool.lrange(key, 0, count - 1)
        if not raw:
            return []
        await self._pool.ltrim(key, count, -1)
        events = []
        for item in raw:
            try:
                events.append(json.loads(item))
            except json.JSONDecodeError:
                continue
        return events

    async def traffic_count(self, spec_id: str) -> int:
        if not self._pool:
            return 0
        return await self._pool.llen(f"{TRAFFIC_KEY_PREFIX}{spec_id}")

    async def get_profile(self, user_id: str) -> dict | None:
        if not self._pool:
            return None
        raw = await self._pool.hgetall(f"{PROFILE_KEY_PREFIX}{user_id}")
        return raw if raw else None

    async def update_profile(self, user_id: str, updates: dict[str, Any]) -> None:
        if not self._pool:
            return
        key = f"{PROFILE_KEY_PREFIX}{user_id}"
        await self._pool.hset(key, mapping=updates)
        await self._pool.expire(key, 86400 * 7)

    async def save_checkpoint(self, thread_id: str, state: dict) -> None:
        if not self._pool:
            return
        key = f"{CHECKPOINT_KEY_PREFIX}{thread_id}"
        await self._pool.set(key, json.dumps(state, default=str), ex=86400)

    async def load_checkpoint(self, thread_id: str) -> dict | None:
        if not self._pool:
            return None
        raw = await self._pool.get(f"{CHECKPOINT_KEY_PREFIX}{thread_id}")
        if raw:
            return json.loads(raw)
        return None


redis_client = RedisClient()
