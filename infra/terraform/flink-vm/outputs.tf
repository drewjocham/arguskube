output "flink_instance_name" {
  description = "Name of the Flink VM instance"
  value       = google_compute_instance.flink.name
}

output "flink_internal_ip" {
  description = "Internal IP of the Flink VM"
  value       = google_compute_instance.flink.network_interface[0].network_ip
}

output "flink_public_ip" {
  description = "Public IP of the Flink VM (if assigned)"
  value       = var.assign_public_ip ? google_compute_address.flink[0].address : null
}

output "flink_webui_url" {
  description = "Flink Web UI URL"
  value       = "http://${google_compute_instance.flink.network_interface[0].network_ip}:8081"
}

output "gateway_url" {
  description = "Flink anomaly detection gateway URL"
  value       = "http://${google_compute_instance.flink.network_interface[0].network_ip}:${var.gateway_port}"
}

output "service_account" {
  description = "Flink VM service account email"
  value       = google_service_account.flink_sa.email
}
