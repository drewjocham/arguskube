# Cloud Run IAM bindings.
#
# - frontend: public-readable so the SPA loads without auth.
# - backend / mcp: callable only by the platform's service account
#   (the frontend in turn calls the backend via the same SA).

resource "google_cloud_run_service_iam_member" "frontend_public" {
  location = google_cloud_run_service.frontend.location
  service  = google_cloud_run_service.frontend.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_service_iam_member" "backend_service_agent" {
  location = google_cloud_run_service.backend.location
  service  = google_cloud_run_service.backend.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${var.service_account_email}"
}

resource "google_cloud_run_service_iam_member" "mcp_service_agent" {
  location = google_cloud_run_service.mcp.location
  service  = google_cloud_run_service.mcp.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${var.service_account_email}"
}
