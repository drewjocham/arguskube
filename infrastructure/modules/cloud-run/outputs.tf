output "backend_url" {
  description = "Backend service URL."
  value       = google_cloud_run_service.backend.status[0].url
}

output "frontend_url" {
  description = "Public frontend URL."
  value       = google_cloud_run_service.frontend.status[0].url
}

output "mcp_url" {
  description = "MCP service URL."
  value       = google_cloud_run_service.mcp.status[0].url
}

output "secrets" {
  description = "Secret Manager secret IDs created by this module. Populate `latest` versions out-of-band before Cloud Run starts."
  value = {
    flink_api_key    = google_secret_manager_secret.flink_api_key.secret_id
    deepseek_api_key = google_secret_manager_secret.deepseek_api_key.secret_id
  }
}
