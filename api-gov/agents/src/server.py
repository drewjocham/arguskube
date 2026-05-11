from __future__ import annotations

import logging

import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from opentelemetry import metrics, trace
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import OTLPMetricExporter
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.view import View
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from pydantic import BaseModel

from src.config import config
from src.database import db
import asyncio

from src.anomaly.batch_worker import batch_loop
from src.anomaly.counters import AnomalyCounters
from src.anomaly.stats import RunningStats
from src.anomaly.structural import FieldTracker
from src.graphs.architect import analyze as architect_analyze
from src.graphs.hacker import generate as hacker_generate
from src.graphs.healer import heal as healer_heal
from src.graphs.orchestrator import orchestrate
from src.graphs.sentinel import ingest as sentinel_ingest
from src.graphs.sentinel import scan as sentinel_scan

logger = logging.getLogger(__name__)

# OpenTelemetry

resource = Resource.create({"service.name": "api-gov-agents"})

if config.otel_endpoint:
    tp = TracerProvider(resource=resource)
    tp.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint=config.otel_endpoint)))
    trace.set_tracer_provider(tp)

    mp = MeterProvider(
        resource=resource,
        metric_readers=[OTLPMetricExporter(endpoint=config.otel_endpoint)],
        views=[View(instrument_name="api-gov.*")],
    )
    metrics.set_meter_provider(mp)
else:
    tp = TracerProvider(resource=resource)
    trace.set_tracer_provider(tp)

meter = metrics.get_meter("api-gov-agents")
tracer = trace.get_tracer("api-gov-agents")

counter_specs_analyzed = meter.create_counter("api-gov.specs.analyzed", unit="1", description="Specs analyzed")
counter_drift_scans = meter.create_counter("api-gov.drift.scans", unit="1", description="Drift scans triggered")
counter_traffic_ingested = meter.create_counter("api-gov.traffic.ingested", unit="1", description="Traffic events ingested")
counter_tests_generated = meter.create_counter("api-gov.tests.generated", unit="1", description="Test cases generated")
counter_heal_suggestions = meter.create_counter("api-gov.heal.suggestions", unit="1", description="Heal suggestions made")
histogram_agent_duration = meter.create_histogram("api-gov.agent.duration", unit="s", description="Graph execution duration")
counter_agent_failures = meter.create_counter("api-gov.agent.failures", unit="1", description="Graph execution failures")

# ── FastAPI app 

app = FastAPI(title="API Agents", version="0.1.0")
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"])

if config.otel_endpoint:
    FastAPIInstrumentor.instrument_app(app)


@app.on_event("startup")
async def startup() -> None:
    await db.connect()
    from src.redis_client import redis_client
    await redis_client.connect()
    asyncio.create_task(batch_loop())
    logger.info("agent service started (batch worker running)")


@app.on_event("shutdown")
async def shutdown() -> None:
    await db.close()
    from src.redis_client import redis_client
    await redis_client.close()


# ── Models ──────────────────────────────────────────────────────


class AnalyzeResponse(BaseModel):
    endpoints: int
    critical_paths: int
    auth_routes: int
    summary: str


class TrafficIngestRequest(BaseModel):
    spec_id: str
    method: str
    path: str
    status_code: int
    request: dict | None = None
    response: dict | None = None
    headers: dict[str, str] | None = None


class TestGenerateRequest(BaseModel):
    endpoint_id: str | None = None
    count: int = 5


class TestCase(BaseModel):
    name: str
    method: str
    path: str
    headers: dict[str, str] = {}
    body: dict | None = None
    expected_status: int = 200
    description: str = ""


class TestGenerateResponse(BaseModel):
    test_cases: list[TestCase]


class HealRequest(BaseModel):
    report: dict


# Endpoints 

@app.post("/analyze/{spec_id}")
async def analyze_spec(spec_id: str) -> AnalyzeResponse:
    with tracer.start_as_current_span("architect.analyze") as span:
        span.set_attribute("spec_id", spec_id)
        try:
            result = await architect_analyze(spec_id)
            counter_specs_analyzed.add(1, {"spec_id": spec_id})
            return AnalyzeResponse(**result)
        except Exception as e:
            counter_agent_failures.add(1, {"agent": "architect", "spec_id": spec_id})
            logger.exception("architect analysis failed")
            raise HTTPException(status_code=500, detail=str(e))


@app.post("/drift/scan/{spec_id}")
async def scan_drift(spec_id: str) -> dict:
    with tracer.start_as_current_span("sentinel.scan") as span:
        span.set_attribute("spec_id", spec_id)
        try:
            reports = await sentinel_scan(spec_id)
            counter_drift_scans.add(1, {"spec_id": spec_id})
            return {"status": "scan_complete", "spec_id": spec_id, "drifts_found": len(reports)}
        except Exception as e:
            counter_agent_failures.add(1, {"agent": "sentinel", "spec_id": spec_id})
            logger.exception("drift scan failed")
            raise HTTPException(status_code=500, detail=str(e))


@app.post("/traffic")
async def ingest_traffic(req: TrafficIngestRequest) -> dict:
    with tracer.start_as_current_span("sentinel.ingest") as span:
        span.set_attribute("spec_id", req.spec_id)
        try:
            await sentinel_ingest(req.model_dump())
            counter_traffic_ingested.add(1, {"spec_id": req.spec_id})
            return {"status": "ingested"}
        except Exception as e:
            counter_agent_failures.add(1, {"agent": "sentinel", "spec_id": req.spec_id})
            logger.exception("traffic ingest failed")
            raise HTTPException(status_code=500, detail=str(e))


@app.post("/tests/generate/{spec_id}")
async def generate_tests(spec_id: str, req: TestGenerateRequest) -> TestGenerateResponse:
    with tracer.start_as_current_span("hacker.generate") as span:
        span.set_attribute("spec_id", spec_id)
        span.set_attribute("endpoint_id", req.endpoint_id or "")
        try:
            test_cases = await hacker_generate(spec_id, req.endpoint_id, req.count)
            counter_tests_generated.add(len(test_cases), {"spec_id": spec_id})
            return TestGenerateResponse(test_cases=[TestCase(**tc) for tc in test_cases])
        except Exception as e:
            counter_agent_failures.add(1, {"agent": "hacker", "spec_id": spec_id})
            logger.exception("test generation failed")
            raise HTTPException(status_code=500, detail=str(e))


@app.post("/heal")
async def heal_drift(req: HealRequest) -> dict:
    with tracer.start_as_current_span("healer.heal") as span:
        span.set_attribute("report_id", req.report.get("id", ""))
        try:
            result = await healer_heal(req.report)
            counter_heal_suggestions.add(1)
            return result
        except Exception as e:
            counter_agent_failures.add(1, {"agent": "healer"})
            logger.exception("heal failed")
            raise HTTPException(status_code=500, detail=str(e))


@app.post("/orchestrate/{spec_id}")
async def run_orchestrator(spec_id: str, action: str = "analyze") -> dict:
    with tracer.start_as_current_span("orchestrator.run") as span:
        span.set_attribute("spec_id", spec_id)
        span.set_attribute("action", action)
        try:
            result = await orchestrate(spec_id, action)
            return result
        except Exception as e:
            counter_agent_failures.add(1, {"agent": "orchestrator", "spec_id": spec_id})
            logger.exception("orchestrator failed")
            raise HTTPException(status_code=500, detail=str(e))


@app.get("/health")
async def health() -> dict:
    return {"status": "ok"}


@app.get("/metrics")
async def metrics_endpoint() -> dict:
    return {"status": "metrics_available_via_otel"}


# Anomaly Endpoints


@app.post("/anomaly/flush")
async def flush_batches() -> dict:
    """Force an immediate batch cycle — drain all pending candidates to LLM."""
    from src.anomaly.batch_worker import run_batch_cycle
    await run_batch_cycle()
    return {"status": "flush_complete"}


@app.get("/anomaly/stats/{spec_id}")
async def get_anomaly_stats(spec_id: str) -> dict:
    """Get running statistics for a spec across all metrics."""
    metrics = ["global_error_rate", "throughput"]
    stats = {}
    for m in metrics:
        stats[m] = await RunningStats.get_stats(spec_id, m)

    traffic_rate = await AnomalyCounters.get_traffic_rate(spec_id)
    endpoint_count = await AnomalyCounters.get_endpoint_cardinality(spec_id)
    status_counts = await AnomalyCounters.get_status_counts(spec_id)
    latency = await AnomalyCounters.get_latency_percentiles(spec_id)
    field_profiles = await FieldTracker.get_all_profiles(spec_id)

    return {
        "spec_id": spec_id,
        "running_stats": stats,
        "traffic_rate_rps": traffic_rate,
        "unique_endpoints": endpoint_count,
        "status_codes": status_counts,
        "latency_ms": latency,
        "field_profiles": field_profiles,
    }


@app.get("/anomaly/latency/{spec_id}")
async def get_latency_analysis(spec_id: str) -> dict:
    """Get latency percentiles and trend for a spec."""
    latency = await AnomalyCounters.get_latency_percentiles(spec_id)
    stat = await RunningStats.get_stats(spec_id, "latency")
    return {"spec_id": spec_id, "latency_ms": latency, "latency_stats": stat}


@app.get("/anomaly/traffic/{spec_id}")
async def get_traffic_analysis(spec_id: str) -> dict:
    """Get traffic rate and throughput stats for a spec."""
    rate = await AnomalyCounters.get_traffic_rate(spec_id)
    stats = await RunningStats.get_stats(spec_id, "throughput")
    return {"spec_id": spec_id, "current_rps": rate, "throughput_stats": stats}


@app.get("/anomaly/fields/{spec_id}")
async def get_field_profiles(spec_id: str) -> dict:
    """Get field presence profiles for all endpoints under a spec."""
    profiles = await FieldTracker.get_all_profiles(spec_id)
    return {"spec_id": spec_id, "endpoints": len(profiles), "field_profiles": profiles}


def main() -> None:
    uvicorn.run("src.server:app", host=config.server_host, port=config.server_port, log_level=config.log_level)


if __name__ == "__main__":
    main()
