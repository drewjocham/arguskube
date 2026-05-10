output "endpoint" {
  description = "OpenAI-compatible base URL. Append /v1/chat/completions for inference."
  value       = "http://${google_compute_address.llm.address}:${var.port}"
}

output "public_ip" {
  description = "Static external IP."
  value       = google_compute_address.llm.address
}

output "instance_name" {
  description = "Compute Engine instance name."
  value       = google_compute_instance.llm.name
}

output "ssh_command" {
  description = "Convenience SSH one-liner once gcloud is authed."
  value       = "gcloud compute ssh ${google_compute_instance.llm.name} --zone=${var.zone} --project=${var.project_id}"
}
