# ──────────────────────────────────────────────────────────────────
# Argus Alert Ingress — Vector Pipeline
# ──────────────────────────────────────────────────────────────────
# Replaces the custom Go HTTP service with Vector.dev's lightweight
# observability pipeline. Receives Anomstack webhooks on :8080 and
# routes them to configured sinks (stdout, S3, GCP PubSub).
#
# Build:
#   docker build -t ghcr.io/drewjocham/argus-alert-ingress .
#
# Run (dev):
#   docker run -p 8080:8080 ghcr.io/drewjocham/argus-alert-ingress
#
# Test:
#   curl -X POST http://localhost:8080/webhooks/anomstack \
#     -H "Content-Type: application/json" \
#     -d '{"title":"test","message":"alert","metric_name":"cpu","threshold":0.8}'
# ──────────────────────────────────────────────────────────────────

FROM timberio/vector:0.44.0-alpine

COPY vector/base.toml /etc/vector/vector.toml

EXPOSE 8080
EXPOSE 8686

USER vector

ENTRYPOINT ["/usr/local/bin/vector"]
CMD ["--config", "/etc/vector/vector.toml"]

LABEL org.opencontainers.image.title="Argus Alert Ingress (Vector)"
LABEL org.opencontainers.image.description="Vector-based alert pipeline replacing the custom Go service"
LABEL org.opencontainers.image.source="https://github.com/drewjocham/argus"
