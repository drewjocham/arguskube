# alert-ingress

Receives anomaly alerts from [Anomstack](/anomstack) via webhook and publishes them to Google Cloud PubSub for consumption by Argus.

## Flow

```
Anomstack ──webhook──► alert-ingress ──PubSub──► Argus
```

## Quick start

```bash
cp .example.env .env
ALERT_INGRESS_MODE=stdout go run ./cmd/main.go
```

Send a test alert:

```bash
curl -X POST http://localhost:8080/webhooks/anomstack \
  -H "Content-Type: application/json" \
  -d '{"title":"test","message":"pod crashloop detected","metric_name":"pod_crashloop","threshold":0.8}'
```

## Production (GCP PubSub)

```bash
export ALERT_INGRESS_MODE=gcp
export GOOGLE_CLOUD_PROJECT=your-project
export PUBSUB_TOPIC=argus-alerts
go run ./cmd/main.go
```

## Docker

```bash
docker build -t alert-ingress .
docker run -p 8080:8080 alert-ingress
```

## Env vars

| Variable | Default | Description |
|----------|---------|-------------|
| `ALERT_INGRESS_MODE` | `stdout` | `stdout` or `gcp` |
| `ALERT_INGRESS_PORT` | `8080` | HTTP port |
| `GOOGLE_CLOUD_PROJECT` | — | GCP project (gcp mode) |
| `PUBSUB_TOPIC` | `argus-alerts` | PubSub topic name (gcp mode) |
