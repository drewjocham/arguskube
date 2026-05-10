locals {
  namespace   = "kubewatcher"
  image_tag   = var.image_tag != "" ? var.image_tag : null
  dev_values  = var.environment == "dev" ? { dev = { enabled = true } } : {}
}

# ── Namespace ─────────────────────────────────────────────────────
resource "kubernetes_namespace" "kubewatcher" {
  metadata {
    name = local.namespace
    labels = {
      "pod-security.kubernetes.io/enforce" = "restricted"
    }
  }
}

# ── DeepSeek Secret ───────────────────────────────────────────────
resource "kubernetes_secret" "backend" {
  count = var.deepseek_api_key != "" ? 1 : 0

  metadata {
    name      = "kubewatcher-backend-secret"
    namespace = local.namespace
  }

  data = {
    "deepseek-api-key" = var.deepseek_api_key
    "argocd-token"     = var.argocd_token
  }

  type = "Opaque"
}

# ── Backend ───────────────────────────────────────────────────────
resource "helm_release" "backend" {
  count = var.helm_backend_enabled ? 1 : 0

  name       = "kubewatcher-backend"
  repository = ""
  chart      = "${path.module}/../helm/kubewatcher-backend"
  namespace  = local.namespace
  version    = "0.1.0"

  values = [
    yamlencode({
      replicaCount = var.environment == "dev" ? 1 : 2

      image = {
        repository = "${var.image_registry}/kubewatcher-backend"
        tag        = local.image_tag
      }

      autoscaling = {
        enabled = var.environment != "dev"
        minReplicas = 2
        maxReplicas = 10
      }

      persistence = {
        enabled = var.environment != "dev"
      }

      config = {
        tier           = var.environment == "prod" ? "pro" : "free"
        argocdURL      = var.argocd_url
        anomstackURL   = var.anomstack_url
        inCluster      = "true"
      }

      env = {
        secret = {
          deepseekAPIKey = var.deepseek_api_key != ""
          argocdToken    = var.argocd_token != ""
        }
      }

      resources = var.environment == "dev" ? {
        limits   = { cpu = "200m", memory = "256Mi" }
        requests = { cpu = "50m", memory = "64Mi" }
      } : {
        limits   = { cpu = "1", memory = "1Gi" }
        requests = { cpu = "200m", memory = "256Mi" }
      }
    })
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Frontend ──────────────────────────────────────────────────────
resource "helm_release" "frontend" {
  count = var.helm_frontend_enabled ? 1 : 0

  name       = "kubewatcher-frontend"
  repository = ""
  chart      = "${path.module}/../helm/kubewatcher-frontend"
  namespace  = local.namespace
  version    = "0.1.0"

  values = [
    yamlencode({
      replicaCount = var.environment == "dev" ? 1 : 2

      image = {
        repository = "${var.image_registry}/kubewatcher-frontend"
        tag        = local.image_tag
      }

      backendService = "kubewatcher-backend"

      autoscaling = {
        enabled = var.environment != "dev"
      }

      resources = var.environment == "dev" ? {
        limits   = { cpu = "100m", memory = "64Mi" }
        requests = { cpu = "25m", memory = "32Mi" }
      } : {}
    })
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Alert Ingress ─────────────────────────────────────────────────
resource "helm_release" "alert_ingress" {
  count = var.helm_alert_ingress_enabled ? 1 : 0

  name       = "kubewatcher-alert-ingress"
  repository = ""
  chart      = "${path.module}/../helm/kubewatcher-alert-ingress"
  namespace  = local.namespace
  version    = "0.1.0"

  values = [
    yamlencode({
      image = {
        repository = "${var.image_registry}/kubewatcher-alert-ingress"
        tag        = local.image_tag
      }

      config = {
        mode = var.environment == "prod" ? "gcp" : "stdout"
      }
    })
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── MCP Server ────────────────────────────────────────────────────
resource "helm_release" "mcp" {
  count = var.helm_mcp_enabled ? 1 : 0

  name       = "kubewatcher-mcp"
  repository = ""
  chart      = "${path.module}/../helm/kubewatcher-mcp"
  namespace  = local.namespace
  version    = "0.1.0"

  values = [
    yamlencode({
      image = {
        repository = "${var.image_registry}/kubewatcher-mcp"
        tag        = local.image_tag
      }

      persistence = {
        enabled = var.environment != "dev"
      }
    })
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Agent (in-cluster DaemonSet) ──────────────────────────────────
resource "helm_release" "agent" {
  count = var.helm_agent_enabled ? 1 : 0

  name       = "kubewatcher-agent"
  repository = ""
  chart      = "${path.module}/../helm/kubewatcher-agent"
  namespace  = local.namespace
  version    = "0.1.0"

  values = [
    yamlencode({
      image = {
        repository = "${var.image_registry}/kubewatcher-agent"
        tag        = local.image_tag
      }

      env = {
        saasToken    = var.deepseek_api_key != ""
        saasServerURL = "http://kubewatcher-backend:8080"
      }
    })
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}

# ── Monitoring Stack ──────────────────────────────────────────────
resource "helm_release" "monitoring" {
  count = var.helm_monitoring_enabled ? 1 : 0

  name       = "kubewatcher-monitoring"
  repository = ""
  chart      = "${path.module}/../helm/kubewatcher-monitoring"
  namespace  = local.namespace
  version    = "0.1.0"

  values = [
    yamlencode({
      kube-prometheus-stack = {
        enabled = true

        prometheus = {
          prometheusSpec = {
            retention = var.environment == "dev" ? "7d" : "30d"

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
                  resources   = { requests = { storage = var.environment == "dev" ? "10Gi" : "50Gi" } }
                }
              }
            }
          }
        }

        grafana = {
          enabled      = true
          adminPassword = "kubewatcher"
          persistence = {
            enabled = true
            size    = var.environment == "dev" ? "5Gi" : "10Gi"
          }
        }
      }

      ollama = {
        enabled = false
      }
    })
  ]

  depends_on = [kubernetes_namespace.kubewatcher]
}
