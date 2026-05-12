"""Performance metrics anomaly detection — latency, error rates, traffic volume, status code ratios."""

from __future__ import annotations

from src.anomaly.counters import AnomalyCounters
from src.anomaly.stats import RunningStats


class MetricsAnomalyDetector:
    """Layer 2 anomaly detection for API performance metrics.
    Tracks latency, error rate, throughput, and status code distribution.
    """

    @staticmethod
    async def detect_latency_anomaly(spec_id: str, latency_ms: float, method: str, path: str) -> dict | None:
        """Detect latency spikes using running z-score."""
        metric = f"latency:{method}:{hash(path) % 10000}"
        stats = await RunningStats.update(spec_id, metric, latency_ms)

        if stats["n"] < 10:
            return None

        z = await RunningStats.z_score(spec_id, metric, latency_ms)
        threshold = stats.get("z_score_threshold", 3.0)

        if abs(z) > threshold:
            severity = "critical" if abs(z) > 6 else "high" if abs(z) > 4.5 else "medium"
            return {
                "severity": severity,
                "category": "latency_anomaly",
                "field": f"{method} {path}",
                "issue": (
                    f"Latency {latency_ms:.0f}ms is {abs(z):.1f}σ above baseline "
                    f"({stats['mean']:.0f}ms ± {stats['stddev']:.0f}ms)"
                ),
                "suggestion": "Check for upstream service degradation, DB slow queries, or deployment issues",
                "score": max(0.1, 1.0 - abs(z) / 12),
                "spec_id": spec_id,
                "source": "metrics_anomaly",
            }
        return None

    @staticmethod
    async def detect_traffic_drop(spec_id: str, window: int = 300) -> dict | None:
        """Detect sudden traffic drops using z-score on request rate."""
        current_rate = await AnomalyCounters.get_traffic_rate(spec_id, window)
        stats = await RunningStats.update(spec_id, "throughput", current_rate)

        if stats["n"] < 10:
            return None

        z = await RunningStats.z_score(spec_id, "throughput", current_rate)
        threshold = stats.get("z_score_threshold", 3.0)

        if z < -threshold:  # negative = drop
            severity = "critical" if abs(z) > 6 else "high"
            return {
                "severity": severity,
                "category": "traffic_drop",
                "field": "all endpoints",
                "issue": (
                    f"Traffic dropped to {current_rate:.1f} req/s "
                    f"({abs(z):.1f}σ below baseline of {stats['mean']:.1f} req/s)"
                ),
                "suggestion": "Check for DNS issues, upstream routing, or API gateway problems",
                "score": max(0.2, 1.0 - abs(z) / 15),
                "spec_id": spec_id,
                "source": "metrics_anomaly",
            }
        return None

    @staticmethod
    async def detect_error_rate_spike(spec_id: str) -> dict | None:
        """Detect error rate spikes across all endpoints."""
        statuses = await AnomalyCounters.get_status_counts(spec_id)
        total = sum(statuses.values())
        if total == 0:
            return None

        error_count = sum(v for k, v in statuses.items() if k >= 400)
        error_rate = error_count / total

        stats = await RunningStats.update(spec_id, "global_error_rate", error_rate)
        if stats["n"] < 10:
            return None

        z = await RunningStats.z_score(spec_id, "global_error_rate", error_rate)
        threshold = stats.get("z_score_threshold", 3.0)

        if z > threshold and error_rate > 0.05:  # >5% error rate AND anomalous
            severity = "critical" if error_rate > 0.2 else "high" if error_rate > 0.1 else "medium"
            status_detail = ", ".join(f"{k}: {v}" for k, v in sorted(statuses.items()) if k >= 400)
            return {
                "severity": severity,
                "category": "error_rate_spike",
                "field": "all endpoints",
                "issue": (
                    f"Error rate {error_rate:.1%} ({z:.1f}σ above baseline {stats['mean']:.1%}) "
                    f"| Statuses: {status_detail}"
                ),
                "suggestion": "Check recent deployments, feature flags, or upstream dependency health",
                "score": max(0.1, 1.0 - error_rate),
                "spec_id": spec_id,
                "source": "metrics_anomaly",
            }
        return None

    @staticmethod
    async def detect_all(spec_id: str, method: str, path: str, status: int, latency_ms: float) -> list[dict]:
        """Run all metric-based anomaly checks. Returns list of anomaly reports."""
        reports: list[dict] = []

        lat = await MetricsAnomalyDetector.detect_latency_anomaly(spec_id, latency_ms, method, path)
        if lat:
            reports.append(lat)

        drop = await MetricsAnomalyDetector.detect_traffic_drop(spec_id)
        if drop:
            reports.append(drop)

        spike = await MetricsAnomalyDetector.detect_error_rate_spike(spec_id)
        if spike:
            reports.append(spike)

        return reports
