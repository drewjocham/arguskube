output "instance_name" {
  description = "Name of the Flink VM instance."
  value       = google_compute_instance.flink.name
}

output "internal_ip" {
  description = "Internal IP of the Flink VM."
  value       = google_compute_instance.flink.network_interface[0].network_ip
}

output "public_ip" {
  description = "Public IP, if one was assigned."
  value       = var.assign_public_ip ? google_compute_address.flink[0].address : null
}

output "webui_url" {
  description = "Flink Web UI URL (internal)."
  value       = "http://${google_compute_instance.flink.network_interface[0].network_ip}:8081"
}

output "gateway_url" {
  description = "Anomaly-detection gateway URL (internal)."
  value       = "http://${google_compute_instance.flink.network_interface[0].network_ip}:${var.gateway_port}"
}

output "service_account" {
  description = "Service account email attached to the instance."
  value       = google_service_account.flink_sa.email
}
