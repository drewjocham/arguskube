"""API drift anomaly detection — detects schema changes, field drift, undocumented fields."""

from __future__ import annotations

import json
from typing import Any

from src.anomaly.counters import AnomalyCounters
from src.anomaly.stats import RunningStats
from src.anomaly.structural import FieldTracker
from src.config import config


class APIDriftDetector:
    """Layer 2 structural anomaly detection for API schemas.
    No LLM — pure statistical/structural checks. Fast enough to run on every event.
    """

    @staticmethod
    async def detect_field_drift(
        spec_id: str,
        method: str,
        path: str,
        observed_fields: list[str],
        defined_fields: list[str],
    ) -> list[dict]:
        """Compare observed fields against defined schema fields.
        Detects: undocumented fields, missing fields, field ratio drops.
        """
        reports = []
        observed_set = set(observed_fields)
        defined_set = set(defined_fields)

        # 1. Undocumented fields — present in response but not in spec
        undocumented = observed_set - defined_set
        for field in undocumented:
            is_new = await FieldTracker.detect_new_fields(spec_id, method, path, [field])
            if is_new:
                reports.append({
                    "severity": "medium",
                    "category": "undocumented_field",
                    "field": field,
                    "issue": f"Field '{field}' appears in response but not in OpenAPI spec",
                    "suggestion": f"Add '{field}' to the spec response schema",
                    "score": 0.6,
                    "spec_id": spec_id,
                    "source": "api_anomaly",
                })

        # 2. Missing fields — in spec but absent from observed
        missing = defined_set - observed_set
        for field in missing:
            profile = await FieldTracker.get_field_profile(spec_id, method, path)
            presence = profile.get(field, 1.0)
            if presence > 0.8:  # field was almost always present before
                reports.append({
                    "severity": "high",
                    "category": "missing_field",
                    "field": field,
                    "issue": f"Field '{field}' defined in spec but missing from response (was {presence:.0%} present)",
                    "suggestion": f"Check if '{field}' was removed from the API response",
                    "score": 0.4,
                    "spec_id": spec_id,
                    "source": "api_anomaly",
                })

        return reports

    @staticmethod
    async def detect_status_anomaly(
        spec_id: str,
        status: int,
        method: str,
        path: str,
    ) -> dict | None:
        """Detect unusual status codes for an endpoint using z-score."""
        # Track error rate per endpoint
        is_error = 1.0 if status >= 400 else 0.0
        metric = f"error_rate:{method}:{hash(path) % 10000}"
        stats = await RunningStats.update(spec_id, metric, is_error)

        if stats["n"] < 10:  # not enough data
            return None

        z = await RunningStats.z_score(spec_id, metric, is_error)
        if abs(z) > stats.get("z_score_threshold", 3.0) and is_error:
            return {
                "severity": "high" if abs(z) > 4 else "medium",
                "category": "status_anomaly",
                "field": f"{method} {path}",
                "issue": f"Status {status} is {abs(z):.1f} stddevs from normal ({stats['mean']:.1%} error rate)",
                "suggestion": "Investigate recent API changes that may have broken this endpoint",
                "score": max(0.1, 1.0 - abs(z) / 10),
                "spec_id": spec_id,
                "source": "api_anomaly",
            }
        return None

    @staticmethod
    async def check_schema_drift(
        spec_id: str,
        method: str,
        path: str,
        content_type: str,
        response_size: int,
    ) -> dict | None:
        """Detect sudden changes in response shape using size as proxy."""
        metric = f"response_size:{method}:{hash(path) % 10000}"
        stats = await RunningStats.update(spec_id, metric, float(response_size))

        if stats["n"] < 10:
            return None

        z = await RunningStats.z_score(spec_id, metric, float(response_size))
        if abs(z) > stats.get("z_score_threshold", 3.0):
            return {
                "severity": "medium" if abs(z) > 4 else "low",
                "category": "schema_size_drift",
                "field": f"{method} {path}",
                "issue": f"Response size {response_size} bytes is {abs(z):.1f} stddevs from mean ({stats['mean']:.0f})",
                "suggestion": "Check if the response schema changed significantly",
                "score": max(0.3, 1.0 - abs(z) / 15),
                "spec_id": spec_id,
                "source": "api_anomaly",
            }
        return None
