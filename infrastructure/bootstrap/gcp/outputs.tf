output "state_bucket" {
  description = "Use this as `bucket` in every live/<env>/gcp-*/backend.tf."
  value       = google_storage_bucket.tfstate.name
}
