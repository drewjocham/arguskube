from __future__ import annotations

import json
import logging
from typing import Any

import psycopg
from psycopg.rows import dict_row

from src.config import config

logger = logging.getLogger(__name__)


class AgentDB:
    def __init__(self) -> None:
        self._pool: psycopg.AsyncConnectionPool | None = None

    async def connect(self) -> None:
        self._pool = await psycopg.AsyncConnectionPool.connect(
            config.database_url,
            min_size=2,
            max_size=10,
            open=False,
        )
        logger.info("agent database pool ready")

    async def close(self) -> None:
        if self._pool:
            await self._pool.close()

    def _conn(self) -> psycopg.AsyncConnection:
        if not self._pool:
            raise RuntimeError("database not connected")
        return self._pool

    async def fetch_spec(self, spec_id: str) -> dict | None:
        async with self._pool.connection() as conn, conn.cursor(row_factory=dict_row) as cur:
            await cur.execute(
                "SELECT id, name, version, content, format, created_at, updated_at FROM api_specs WHERE id = %s",
                (spec_id,),
            )
            return await cur.fetchone()

    async def fetch_endpoints(self, spec_id: str) -> list[dict]:
        async with self._pool.connection() as conn, conn.cursor(row_factory=dict_row) as cur:
            await cur.execute(
                "SELECT id, method, path, summary, operation_id, request_body, responses, parameters, security, tags "
                "FROM endpoints WHERE spec_id = %s ORDER BY method, path",
                (spec_id,),
            )
            return await cur.fetchall() or []

    async def fetch_pending_drifts(self, spec_id: str, limit: int = 20) -> list[dict]:
        async with self._pool.connection() as conn, conn.cursor(row_factory=dict_row) as cur:
            await cur.execute(
                "SELECT id, endpoint_id, severity, category, score, source, observed, expected, actual, suggestion, created_at "
                "FROM drift_reports WHERE spec_id = %s AND resolved = FALSE ORDER BY score ASC LIMIT %s",
                (spec_id, limit),
            )
            return await cur.fetchall() or []

    async def save_drift_reports(self, reports: list[dict]) -> None:
        if not reports:
            return
        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                for r in reports:
                    await cur.execute(
                        "INSERT INTO drift_reports (spec_id, endpoint_id, severity, category, score, "
                        "source, observed, expected, actual, suggestion, created_at) "
                        "VALUES (%(spec_id)s, %(endpoint_id)s, %(severity)s, %(category)s, %(score)s, "
                        "%(source)s, %(observed)s, %(expected)s, %(actual)s, %(suggestion)s, NOW())",
                        r,
                    )

    async def save_generated_tests(self, spec_id: str, tests: list[dict]) -> None:
        if not tests:
            return
        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                for t in tests:
                    await cur.execute(
                        "INSERT INTO generated_tests (spec_id, endpoint_id, name, method, path, "
                        "headers, body, expected_status, description, created_at) "
                        "VALUES (%(spec_id)s, %(endpoint_id)s, %(name)s, %(method)s, %(path)s, "
                        "%(headers)s, %(body)s, %(expected_status)s, %(description)s, NOW())",
                        {**t, "spec_id": spec_id},
                    )

    async def save_llm_usage(
        self, spec_id: str, agent: str, model: str,
        prompt_tokens: int, completion_tokens: int, duration_ms: int,
    ) -> None:
        async with self._pool.connection() as conn:
            await conn.execute(
                "INSERT INTO llm_usage (spec_id, agent, model, prompt_tokens, completion_tokens, total_tokens, duration_ms) "
                "VALUES ($1, $2, $3, $4, $5, $6, $7)",
                spec_id, agent, model, prompt_tokens, completion_tokens, prompt_tokens + completion_tokens, duration_ms,
            )

    async def save_investigation(
        self, spec_id: str, agent_name: str, result: str, confidence: float, log: dict,
    ) -> None:
        async with self._pool.connection() as conn:
            await conn.execute(
                "INSERT INTO investigations (spec_id, agent_name, result, confidence, investigation_log, completed_at) "
                "VALUES ($1, $2, $3, $4, $5, NOW())",
                spec_id, agent_name, result, confidence, json.dumps(log),
            )

    async def save_feedback(self, alert_id: str, spec_id: str, action: str, notes: str = "") -> None:
        async with self._pool.connection() as conn:
            await conn.execute(
                "INSERT INTO user_feedback (alert_id, spec_id, action, notes) VALUES ($1, $2, $3, $4)",
                alert_id, spec_id, action, notes,
            )

    async def save_anomaly_metric(
        self, spec_id: str, date: str, total_alerts: int,
        true_positives: int, false_positives: int,
    ) -> None:
        precision = true_positives / max(true_positives + false_positives, 1)
        recall = true_positives / max(total_alerts, 1)
        async with self._pool.connection() as conn:
            await conn.execute(
                "INSERT INTO anomaly_metrics (spec_id, date, total_alerts, true_positives, false_positives, precision, recall) "
                "VALUES ($1, $2, $3, $4, $5, $6, $7) "
                "ON CONFLICT (spec_id, date) DO UPDATE SET "
                "total_alerts = EXCLUDED.total_alerts, true_positives = EXCLUDED.true_positives, "
                "false_positives = EXCLUDED.false_positives, precision = EXCLUDED.precision, recall = EXCLUDED.recall",
                spec_id, date, total_alerts, true_positives, false_positives, round(precision, 4), round(recall, 4),
            )


db = AgentDB()
