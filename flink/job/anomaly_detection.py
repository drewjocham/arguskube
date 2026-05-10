"""PyFlink anomaly detection job for KubeWatcher.

Consumes Kubernetes metric streams and detects anomalies using
moving average, z-score, and change point detection algorithms.

Deployed as a Flink job on the Flink cluster (VM). Job receives
metrics from the gateway service or directly from Prometheus.
"""

from pyflink.datastream import StreamExecutionEnvironment
from pyflink.datastream.connectors.kafka import FlinkKafkaConsumer
from pyflink.common.serialization import SimpleStringSchema
from pyflink.common.typeinfo import Types
from pyflink.common import Time
from pyflink.datastream.window import TumblingEventTimeWindows

import json
import statistics
from collections import deque
from typing import List, Dict, Optional


# ─── Configuration ───────────────────────────────────────────────────
KAFKA_BOOTSTRAP_SERVERS = "localhost:9092"
KAFKA_GROUP_ID = "kubewatcher-flink"
INPUT_TOPIC = "k8s_metrics"
OUTPUT_TOPIC = "kubewatcher_anomalies"
WINDOW_SIZE_SECONDS = 60
Z_SCORE_THRESHOLD = 2.5
SMA_WINDOW_SIZE = 5


# ─── Anomaly Detection Algorithms ────────────────────────────────────

class MovingAverageDetector:
    """Simple Moving Average (SMA) anomaly detector.

    Compares current value to rolling average. Deviation beyond
    the threshold triggers an anomaly.
    """

    def __init__(self, window_size: int = SMA_WINDOW_SIZE, threshold: float = Z_SCORE_THRESHOLD):
        self.window = deque(maxlen=window_size)
        self.threshold = threshold

    def detect(self, value: float) -> tuple[bool, float, str]:
        self.window.append(value)
        if len(self.window) < 3:
            return False, 0.0, "insufficient_data"

        mean = statistics.mean(self.window)
        stdev = statistics.stdev(self.window) if len(self.window) > 1 else 0.0

        if stdev == 0.0:
            return False, 0.0, "no_variance"

        z_score = abs(value - mean) / stdev
        is_anomaly = z_score > self.threshold
        score = min(z_score / 5.0, 1.0)

        if is_anomaly:
            description = (
                f"Value {value:.2f} deviates {z_score:.1f}s from mean {mean:.2f}"
            )
            return is_anomaly, score, description

        return False, score, "normal"


class ChangePointDetector:
    """Detects sudden shifts in metric patterns using CUSUM."""

    def __init__(self, threshold: float = 3.0, drift: float = 0.5):
        self.mean = 0.0
        self.cusum_pos = 0.0
        self.cusum_neg = 0.0
        self.threshold = threshold
        self.drift = drift
        self.count = 0

    def detect(self, value: float) -> tuple[bool, float, str]:
        self.count += 1
        if self.count == 1:
            self.mean = value
            return False, 0.0, "baseline"

        delta = value - self.mean
        self.mean = self.mean + delta / min(self.count, 100)

        self.cusum_pos = max(0.0, self.cusum_pos + delta - self.drift)
        self.cusum_neg = min(0.0, self.cusum_neg + delta + self.drift)

        if self.cusum_pos > self.threshold:
            score = min(self.cusum_pos / (self.threshold * 2), 1.0)
            return True, score, f"Positive change point detected (delta={delta:.2f})"

        if self.cusum_neg < -self.threshold:
            score = min(abs(self.cusum_neg) / (self.threshold * 2), 1.0)
            return True, score, f"Negative change point detected (delta={delta:.2f})"

        return False, 0.0, "stable"


# ─── Flink Stream Processing ─────────────────────────────────────────

class MetricAnomalyDetector:
    """Maps raw metrics to anomaly scores using multiple algorithms."""

    def __init__(self):
        self.sma = MovingAverageDetector()
        self.cpd = ChangePointDetector()

    def process_metric(self, metric: Dict) -> Dict:
        value = metric.get("value", 0.0)
        metric_name = metric.get("metric_name", "unknown")
        labels = metric.get("labels", {})

        sma_result, sma_score, sma_desc = self.sma.detect(value)
        cpd_result, cpd_score, cpd_desc = self.cpd.detect(value)

        is_anomaly = sma_result or cpd_result
        score = max(sma_score, cpd_score)
        description = cpd_desc if cpd_result else sma_desc

        return {
            "metric_name": metric_name,
            "labels": labels,
            "value": value,
            "is_anomaly": is_anomaly,
            "score": round(score, 4),
            "description": description,
            "algorithms": {
                "sma_anomaly": sma_result,
                "cpd_anomaly": cpd_result,
            },
        }


def parse_metric(message: str) -> Optional[Dict]:
    try:
        return json.loads(message)
    except json.JSONDecodeError:
        return None


def main():
    env = StreamExecutionEnvironment.get_execution_environment()
    env.set_parallelism(2)

    consumer = FlinkKafkaConsumer(
        topics=INPUT_TOPIC,
        deserialization_schema=SimpleStringSchema(),
        properties={
            "bootstrap.servers": KAFKA_BOOTSTRAP_SERVERS,
            "group.id": KAFKA_GROUP_ID,
        },
    )

    ds = env.add_source(consumer)

    def process_stream(message: str) -> str:
        metric = parse_metric(message)
        if metric is None:
            return ""

        detector = MetricAnomalyDetector()
        result = detector.process_metric(metric)
        return json.dumps(result)

    (
        ds.map(process_stream, output_type=Types.STRING)
          .filter(lambda x: x != "")
          .map(lambda x: x, output_type=Types.STRING)
    )

    env.execute("kubewatcher-flink-anomaly-detection")


if __name__ == "__main__":
    main()
