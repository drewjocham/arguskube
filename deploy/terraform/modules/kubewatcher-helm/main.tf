locals {
  ns        = kubernetes_namespace.kubewatcher.metadata[0].name
  image_tag = var.image_tag != "" ? var.image_tag : null

  # Resource sizing keyed on environment. Single source of truth so a
  # bug in dev-tier sizing can't drift away from prod's defaults.
  sizing = {
    dev = {
      replicas = 1
      backend = {
        limits   = { cpu = "200m", memory = "256Mi" }
        requests = { cpu = "50m", memory = "64Mi" }
      }
      frontend = {
        limits   = { cpu = "100m", memory = "64Mi" }
        requests = { cpu = "25m", memory = "32Mi" }
      }
      prom_retention = "7d"
      prom_storage   = "10Gi"
      grafana_pvc    = "5Gi"
    }
    staging = {
      replicas = 2
      backend = {
        limits   = { cpu = "500m", memory = "512Mi" }
        requests = { cpu = "100m", memory = "128Mi" }
      }
      frontend = {
        limits   = { cpu = "200m", memory = "128Mi" }
        requests = { cpu = "50m", memory = "64Mi" }
      }
      prom_retention = "15d"
      prom_storage   = "20Gi"
      grafana_pvc    = "5Gi"
    }
    prod = {
      replicas = 2
      backend = {
        limits   = { cpu = "1", memory = "1Gi" }
        requests = { cpu = "200m", memory = "256Mi" }
      }
      frontend = {
        limits   = { cpu = "500m", memory = "256Mi" }
        requests = { cpu = "100m", memory = "128Mi" }
      }
      prom_retention = "30d"
      prom_storage   = "50Gi"
      grafana_pvc    = "10Gi"
    }
  }
  s = local.sizing[var.environment]

  feature_tier = var.environment == "prod" ? "pro" : "free"
}

# ── Backend ───────────────────────────────────────────────────────
resource "helm_release" "backend" {
  count = var.backend_enabled ? 1 : 0

  name      = "kubewatcher-backend"
  chart     = "${var.chart_path}/kubewatcher-backend"
  namespace = local.ns
  version   = "0.1.0"

  values = [
    yamlencode(merge({
      replicaCount = local.s.replicas

      image = {
        repository = "${var.image_registry}/kubewatcher-backend"
        tag        = local.image_tag
      }

      autoscaling = {
        enabled     = var.environment != "dev"
        minReplicas = local.s.replicas
        maxReplicas = 10
      }

      persistence = {
        enabled = var.environment != "dev"
      }

      config = {
        tier         = local.feature_tier
        argocdURL    = var.argocd_url
        anomstackURL = var.anomstack_url
        inCluster    = "true"
      }

      env = {
        secret = {
          deepseekAPIKey = var.deepseek_api_key != ""
          argocdToken    = var.argocd_token != ""
        }
      }

      resources = local.s.backend
    }, var.extra_values_backend))
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Frontend ──────────────────────────────────────────────────────
resource "helm_release" "frontend" {
  count = var.frontend_enabled ? 1 : 0

  name      = "kubewatcher-frontend"
  chart     = "${var.chart_path}/kubewatcher-frontend"
  namespace = local.ns
  version   = "0.1.0"

  values = [
    yamlencode(merge({
      replicaCount = local.s.replicas

      image = {
        repository = "${var.image_registry}/kubewatcher-frontend"
        tag        = local.image_tag
      }

      backendService = "kubewatcher-backend"

      autoscaling = {
        enabled = var.environment != "dev"
      }

      resources = local.s.frontend
    }, var.extra_values_frontend))
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Alert Ingress ─────────────────────────────────────────────────
resource "helm_release" "alert_ingress" {
  count = var.alert_ingress_enabled ? 1 : 0

  name      = "kubewatcher-alert-ingress"
  chart     = "${var.chart_path}/kubewatcher-alert-ingress"
  namespace = local.ns
  version   = "0.2.0"

  values = [
    yamlencode(merge({
      image = {
        repository = "${var.image_registry}/kubewatcher-alert-ingress"
        tag        = local.image_tag
      }

      vector = {
        logLevel = var.environment == "dev" ? "debug" : "info"
      }
      sinks = {
        stdout = { enabled = true }
      }
    }, var.extra_values_alert_ingress))
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── MCP Server ────────────────────────────────────────────────────
resource "helm_release" "mcp" {
  count = var.mcp_enabled ? 1 : 0

  name      = "kubewatcher-mcp"
  chart     = "${var.chart_path}/kubewatcher-mcp"
  namespace = local.ns
  version   = "0.1.0"

  values = [
    yamlencode(merge({
      image = {
        repository = "${var.image_registry}/kubewatcher-mcp"
        tag        = local.image_tag
      }

      persistence = {
        enabled = var.environment != "dev"
      }
    }, var.extra_values_mcp))
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Agent (in-cluster DaemonSet) ──────────────────────────────────
resource "helm_release" "agent" {
  count = var.agent_enabled ? 1 : 0

  name      = "kubewatcher-agent"
  chart     = "${var.chart_path}/kubewatcher-agent"
  namespace = local.ns
  version   = "0.1.0"

  values = [
    yamlencode(merge({
      image = {
        repository = "${var.image_registry}/kubewatcher-agent"
        tag        = local.image_tag
      }

      env = {
        saasToken     = var.deepseek_api_key != ""
        saasServerURL = "http://kubewatcher-backend:8080"
      }
    }, var.extra_values_agent))
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Monitoring stack (kube-prometheus-stack + Grafana) ────────────
resource "helm_release" "monitoring" {
  count = var.monitoring_enabled ? 1 : 0

  name      = "kubewatcher-monitoring"
  chart     = "${var.chart_path}/kubewatcher-monitoring"
  namespace = local.ns
  version   = "0.2.0"

  values = [
    yamlencode(merge({
      kube-prometheus-stack = {
        enabled = true

        prometheus = {
          prometheusSpec = {
            retention = local.s.prom_retention

            resources = var.environment == "dev" ? {
              requests = { cpu = "200m", memory = "512Mi" }
              limits   = { cpu = "500m", memory = "1Gi" }
              } : {
              requests = { cpu = "500m", memory = "2Gi" }
              limits   = { cpu = "2", memory = "4Gi" }
            }

            storageSpec = {
              volumeClaimTemplate = {
                spec = {
                  accessModes = ["ReadWriteOnce"]
                  resources   = { requests = { storage = local.s.prom_storage } }
                }
              }
            }
          }
        }

        # Grafana admin password: required and never defaulted in code.
        # The chart's own default leaves it blank — set
        # var.grafana_admin_password (ideally from a tfvars file or the
        # secret manager of your choice).
        grafana = {
          enabled       = true
          adminPassword = var.grafana_admin_password
          persistence = {
            enabled = true
            size    = local.s.grafana_pvc
          }
        }
      }

    }, var.extra_values_monitoring))
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}
