# ── Backend API ───────────────────────────────────────────────────
resource "google_cloud_run_service" "backend" {
  name     = "${var.name_prefix}-backend"
  location = var.region

  template {
    spec {
      containers {
        image = var.backend_image
        ports {
          container_port = var.backend_port
        }

        env {
          name  = "KUBEWATCHER_TIER"
          value = var.tier
        }
        env {
          name  = "KUBEWATCHER_LOG_LEVEL"
          value = var.log_level
        }
        env {
          name  = "KUBEWATCHER_LOG_FORMAT"
          value = var.log_format
        }
        env {
          name  = "KUBEWATCHER_PORT"
          value = var.backend_port
        }
        env {
          name  = "KUBEWATCHER_FLINK_URL"
          value = var.flink_gateway_url
        }
        env {
          name = "KUBEWATCHER_FLINK_API_KEY"
          value_from {
            secret_key_ref {
              name = google_secret_manager_secret.flink_api_key.secret_id
              key  = "latest"
            }
          }
        }
        env {
          name  = "KUBEWATCHER_IN_CLUSTER"
          value = "false"
        }
        env {
          name = "DEEPSEEK_API_KEY"
          value_from {
            secret_key_ref {
              name = google_secret_manager_secret.deepseek_api_key.secret_id
              key  = "latest"
            }
          }
        }
        env {
          name  = "PROMETHEUS_URL"
          value = var.prometheus_url
        }

        resources {
          limits = {
            cpu    = var.backend_cpu
            memory = var.backend_memory
          }
        }

        startup_probe {
          http_get {
            path = "/healthz"
            port = var.backend_port
          }
          initial_delay_seconds = 10
          timeout_seconds       = 5
          period_seconds        = 10
        }
      }

      service_account_name = var.service_account_email
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale"        = var.backend_min_instances
        "autoscaling.knative.dev/maxScale"        = var.backend_max_instances
        "run.googleapis.com/cloudsql-instances"   = var.cloudsql_instance_name
        "run.googleapis.com/vpc-access-connector" = var.vpc_connector
        "run.googleapis.com/ingress"              = "all"
        "run.googleapis.com/ingress-status"       = "all"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  autogenerate_revision_name = true

  depends_on = [
    google_secret_manager_secret.flink_api_key,
    google_secret_manager_secret.deepseek_api_key,
  ]
}

# ── Frontend SPA ──────────────────────────────────────────────────
resource "google_cloud_run_service" "frontend" {
  name     = "${var.name_prefix}-frontend"
  location = var.region

  template {
    spec {
      containers {
        image = var.frontend_image
        ports {
          container_port = 8080
        }

        env {
          name  = "BACKEND_URL"
          value = google_cloud_run_service.backend.status[0].url
        }

        resources {
          limits = {
            cpu    = var.frontend_cpu
            memory = var.frontend_memory
          }
        }
      }

      service_account_name = var.service_account_email
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale" = var.frontend_min_instances
        "autoscaling.knative.dev/maxScale" = var.frontend_max_instances
        "run.googleapis.com/ingress"       = "all"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  autogenerate_revision_name = true
}

# ── MCP service ───────────────────────────────────────────────────
resource "google_cloud_run_service" "mcp" {
  name     = "${var.name_prefix}-mcp"
  location = var.region

  template {
    spec {
      containers {
        image = var.mcp_image
        ports {
          container_port = 9090
        }

        env {
          name  = "FLINK_URL"
          value = var.flink_gateway_url
        }
        env {
          name = "FLINK_API_KEY"
          value_from {
            secret_key_ref {
              name = google_secret_manager_secret.flink_api_key.secret_id
              key  = "latest"
            }
          }
        }

        resources {
          limits = {
            cpu    = var.mcp_cpu
            memory = var.mcp_memory
          }
        }
      }

      service_account_name = var.service_account_email
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale" = var.mcp_min_instances
        "autoscaling.knative.dev/maxScale" = var.mcp_max_instances
        "run.googleapis.com/ingress"       = "all"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  autogenerate_revision_name = true
}
