locals {
  labels = merge(var.tags, {
    argus-managed = "true"
    argus-run-id  = var.run_id
  })

  # ── Broker port by kind ─────────────────────────────────────────
  broker_port = var.broker_kind == "nats"     ? 4222
              : var.broker_kind == "kafka"    ? 9092
              : var.broker_kind == "rabbitmq" ? 5672
              : var.broker_kind == "amqp1"    ? 5671
              : var.broker_kind == "rest"     ? 80
              : 4222

  # ── In-cluster endpoint (engine connects here) ──────────────────
  broker_endpoint = var.broker_kind == "nats"  ? "nats://broker-${var.run_id}:${local.broker_port}"
                  : var.broker_kind == "kafka" ? "broker-${var.run_id}:${local.broker_port}"
                  : var.broker_kind == "rabbitmq" ? "amqp://broker-${var.run_id}:${local.broker_port}"
                  : var.broker_kind == "amqp1" ? "amqps://broker-${var.run_id}:${local.broker_port}"
                  : var.broker_kind == "rest"  ? "http://broker-${var.run_id}:${local.broker_port}"
                  : ""  # pubsub uses GCP service directly

  # ── Helm chart by kind ──────────────────────────────────────────
  helm_repo    = local.helm_repos[var.broker_kind]
  helm_chart   = local.helm_charts[var.broker_kind]
  helm_version = lookup(local.helm_versions, var.broker_kind, "")

  helm_repos = {
    nats     = "https://nats-io.github.io/k8s/helm/charts/"
    kafka    = "https://charts.bitnami.com/bitnami"
    rabbitmq = "https://charts.bitnami.com/bitnami"
    amqp1    = "https://azure.github.io/azure-service-bus-charts/"  # placeholder
  }

  helm_charts = {
    nats     = "nats"
    kafka    = "kafka"
    rabbitmq = "rabbitmq"
    amqp1    = "solace-pubsub"  # placeholder
  }

  helm_versions = {
    nats     = ""
    kafka    = ""
    rabbitmq = ""
    amqp1    = ""
  }

  # ── Per-kind Helm overrides ─────────────────────────────────────
  helm_values = merge(
    var.broker_kind == "nats" ? {
      "config.nats.max_payload" = "8MB"
    } : {},
    var.broker_kind == "kafka" ? {
      "listeners.client.protocol" = "PLAINTEXT"
    } : {},
    var.broker_kind == "rabbitmq" ? {
      "auth.username" = "runner"
      "auth.password" = "runner"
    } : {},
  )

  # External endpoint for pubsub (no in-cluster broker).
  broker_external_endpoint = var.broker_kind == "pubsub" ? var.project_id : ""
}
