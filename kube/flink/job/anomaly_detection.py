from pyflink.datastream import StreamExecutionEnvironment
from pyflink.datastream.connectors.kafka import FlinkKafkaConsumer, FlinkKafkaProducer
from pyflink.common.serialization import SimpleStringSchema
from pyflink.common.typeinfo import Types
from pyflink.datastream.functions import KeyedProcessFunction
from pyflink.datastream.state import ValueStateDescriptor
from pyflink.common.time import Time

import json
import statistics
from typing import Optional, Dict

# Configuration 
KAFKA_BOOTSTRAP_SERVERS = "localhost:9092"
KAFKA_GROUP_ID = "argus-flink"
INPUT_TOPIC = "k8s_metrics"
OUTPUT_TOPIC = "argus_anomalies"

# Algorithm Tuning
WINDOW_SIZE_SECONDS = 60
Z_SCORE_THRESHOLD = 2.5
SMA_WINDOW_SIZE = 5
CPD_THRESHOLD = 3.0
CPD_DRIFT = 0.5


# Flink Stateful Stream Processing

class StatefulAnomalyDetector(KeyedProcessFunction):
    """
    Evaluates metrics for anomalies using Flink's Managed State.
    This ensures that each metric (e.g., CPU, Memory) maintains its own 
    isolated historical baseline across the distributed cluster.
    """

    def __init__(self):
        self.sma_state = None
        self.cpd_state = None

    def open(self, runtime_context):
        # Initialize Flink state variables to persist history
        # We store the state as JSON strings for easy serialization
        sma_descriptor = ValueStateDescriptor("sma_history", Types.STRING())
        self.sma_state = runtime_context.get_state(sma_descriptor)
        
        cpd_descriptor = ValueStateDescriptor("cpd_history", Types.STRING())
        self.cpd_state = runtime_context.get_state(cpd_descriptor)

    def process_element(self, msg_str: str, ctx: 'KeyedProcessFunction.Context'):
        try:
            metric = json.loads(msg_str)
        except json.JSONDecodeError:
            return  # Drop malformed messages

        value = metric.get("value", 0.0)
        metric_name = metric.get("metric_name", "unknown")
        labels = metric.get("labels", {})

        # Simple Moving Average (SMA) Logic ---
        sma_history_str = self.sma_state.value()
        sma_window = json.loads(sma_history_str) if sma_history_str else []
        
        sma_window.append(value)
        if len(sma_window) > SMA_WINDOW_SIZE:
            sma_window.pop(0)
            
        # Update SMA state
        self.sma_state.update(json.dumps(sma_window))

        sma_result, sma_score, sma_desc = False, 0.0, "normal"
        if len(sma_window) >= 3:
            mean = statistics.mean(sma_window)
            stdev = statistics.stdev(sma_window) if len(sma_window) > 1 else 0.0
            
            if stdev > 0.0:
                z_score = abs(value - mean) / stdev
                sma_result = z_score > Z_SCORE_THRESHOLD
                sma_score = min(z_score / 5.0, 1.0)
                if sma_result:
                    sma_desc = f"Value {value:.2f} deviates {z_score:.1f}s from mean {mean:.2f}"
            else:
                sma_desc = "no_variance"
        else:
            sma_desc = "insufficient_data"


        # Change Point Detection (CUSUM) Logic 
        cpd_state_str = self.cpd_state.value()
        cpd = json.loads(cpd_state_str) if cpd_state_str else {"mean": 0.0, "cusum_pos": 0.0, "cusum_neg": 0.0, "count": 0}
        
        cpd["count"] += 1
        cpd_result, cpd_score, cpd_desc = False, 0.0, "stable"

        if cpd["count"] == 1:
            cpd["mean"] = value
            cpd_desc = "baseline"
        else:
            delta = value - cpd["mean"]
            cpd["mean"] = cpd["mean"] + delta / min(cpd["count"], 100)

            cpd["cusum_pos"] = max(0.0, cpd["cusum_pos"] + delta - CPD_DRIFT)
            cpd["cusum_neg"] = min(0.0, cpd["cusum_neg"] + delta + CPD_DRIFT)

            if cpd["cusum_pos"] > CPD_THRESHOLD:
                cpd_result = True
                cpd_score = min(cpd["cusum_pos"] / (CPD_THRESHOLD * 2), 1.0)
                cpd_desc = f"Positive change point detected (delta={delta:.2f})"
            elif cpd["cusum_neg"] < -CPD_THRESHOLD:
                cpd_result = True
                cpd_score = min(abs(cpd["cusum_neg"]) / (CPD_THRESHOLD * 2), 1.0)
                cpd_desc = f"Negative change point detected (delta={delta:.2f})"

        # Update Flink CPD state
        self.cpd_state.update(json.dumps(cpd))

        # Aggregate Results and Yield ---
        is_anomaly = sma_result or cpd_result
        
        # We can optionally filter here to ONLY output anomalies to save bandwidth,
        # but emitting everything is fine if you want the scores.
        if is_anomaly:
            score = max(sma_score, cpd_score)
            description = cpd_desc if cpd_result else sma_desc

            output = {
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
            
            # Flink process functions use 'yield' to emit records downstream
            yield json.dumps(output)


def get_routing_key(msg_str: str) -> str:
    """Extracts the metric name to use as a partition key."""
    try:
        metric = json.loads(msg_str)
        # Keying by metric_name ensures all memory metrics go to the same state window,
        # and all CPU metrics go to a different state window.
        # Note: If metrics are per-pod, you might want to return: f'{metric["metric_name"]}_{metric["labels"].get("pod")}'
        return metric.get("metric_name", "unknown")
    except Exception:
        return "unknown"


def main():
    env = StreamExecutionEnvironment.get_execution_environment()
    env.set_parallelism(2) # Adjust based on your cluster

    # 1. Source: Read from Kafka
    consumer = FlinkKafkaConsumer(
        topics=INPUT_TOPIC,
        deserialization_schema=SimpleStringSchema(),
        properties={
            "bootstrap.servers": KAFKA_BOOTSTRAP_SERVERS,
            "group.id": KAFKA_GROUP_ID,
        },
    )
    
    # 2. Sink: Write back to Kafka
    producer = FlinkKafkaProducer(
        topic=OUTPUT_TOPIC,
        serialization_schema=SimpleStringSchema(),
        producer_config={
            "bootstrap.servers": KAFKA_BOOTSTRAP_SERVERS
        }
    )

    stream = env.add_source(consumer, "Kafka Source")

    # 3. The Pipeline Execution
    (
        stream
        .filter(lambda msg: msg is not None)
        # Group by the specific metric so state is calculated independently
        .key_by(get_routing_key, key_type=Types.STRING())
        # Apply the stateful anomaly logic
        .process(StatefulAnomalyDetector(), output_type=Types.STRING())
        # Output the flagged anomalies to the destination topic
        .add_sink(producer)
    )

    env.execute("argus-flink-anomaly-detection")


if __name__ == "__main__":
    main()