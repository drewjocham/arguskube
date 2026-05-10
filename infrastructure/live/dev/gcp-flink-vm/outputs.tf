output "gateway_url" {
  description = "Internal URL for the Flink anomaly-detection gateway."
  value       = module.flink.gateway_url
}

output "service_account" {
  description = "Service account email attached to the VM."
  value       = module.flink.service_account
}

output "internal_ip" {
  description = "Internal IP of the Flink VM."
  value       = module.flink.internal_ip
}
