output "namespace" {
  description = "Kubernetes namespace where every release lands."
  value       = local.ns
}

output "backend_service" {
  description = "Cluster DNS name of the backend service."
  value       = "argus-backend.${local.ns}.svc.cluster.local"
}

output "frontend_service" {
  description = "Cluster DNS name of the frontend service."
  value       = "argus-frontend.${local.ns}.svc.cluster.local"
}

output "releases" {
  description = "Map of release name → enabled flag, useful for downstream conditionals and debugging."
  value = {
    backend       = var.backend_enabled
    frontend      = var.frontend_enabled
    agent         = var.agent_enabled
    alert_ingress = var.alert_ingress_enabled
    mcp           = var.mcp_enabled
    monitoring    = var.monitoring_enabled
  }
}
