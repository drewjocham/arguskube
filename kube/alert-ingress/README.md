# alert-ingress

Receives anomaly alerts from [Anomstack](/anomstack) via webhook and routes them to configured sinks (stdout, S3, GCP PubSub).

**Replaced the custom Go service with [Vector.dev](https://vector.dev/) — no more custom `cmd/main.go`.**

## Flow

```
Anomstack ──webhook──► Vector (alert-ingress) ──sinks──► stdout / S3 / GCP PubSub
```

## Quick start

```bash
docker build -t alert-ingress .
docker run -p 8080:8080 alert-ingress
```

Send a test alert:

```bash
curl -X POST http://localhost:8080/webhooks/anomstack \
  -H "Content-Type: application/json" \
  -d '{"title":"test","message":"pod crashloop detected","metric_name":"pod_crashloop","threshold":0.8}'
```

## Sinks

| Sink | Enable via | Description |
|------|-----------|-------------|
| **stdout** | Always on | Writes structured JSON to stdout for container logs |
| **S3** | Set `S3_ALERTS_BUCKET` + `AWS_REGION` | Archives alerts to S3 as gzipped JSON |
| **GCP PubSub** | Set `GOOGLE_CLOUD_PROJECT` + `PUBSUB_TOPIC` | Publishes alerts to GCP PubSub topic |

## Testing the Vector config

```bash
# Validate config syntax
vector validate vector/base.toml --no-environment

# Run transform tests
./vector/tests/test_transform.sh
```

## Env vars

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `S3_ALERTS_BUCKET` | For S3 sink | — | S3 bucket for alert archival |
| `AWS_REGION` | For S3 sink | — | AWS region |
| `GOOGLE_CLOUD_PROJECT` | For GCP sink | — | GCP project ID |
| `PUBSUB_TOPIC` | For GCP sink | `argus-alerts` | GCP PubSub topic name |

## Architecture

The old alert-ingress was a ~150-line Go HTTP server with a GCP PubSub publisher interface.
It's now replaced by a ~50-line Vector TOML config that is:

- **Smaller**: ~50 lines of config vs ~200 lines of Go code
- **Extensible**: Add new sinks without code changes
- **Vendor-neutral**: Swap S3 ↔ GCS ↔ Kafka ↔ HTTP with a config change
- **Zero maintenance**: Vector is a mature open-source project with regular releases
