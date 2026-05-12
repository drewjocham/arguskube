output "cluster_name" {
  description = "GKE cluster name."
  value       = google_container_cluster.primary.name
}

output "cluster_endpoint" {
  description = "GKE cluster API endpoint (hostname or IP)."
  value       = google_container_cluster.primary.endpoint
}

output "cluster_ca_certificate" {
  description = "Base64-encoded CA certificate for the cluster API."
  value       = google_container_cluster.primary.master_auth[0].cluster_ca_certificate
}

output "cluster_location" {
  description = "Region the cluster lives in."
  value       = google_container_cluster.primary.location
}

output "vpc_name" {
  description = "VPC the cluster lives in."
  value       = google_compute_network.vpc.name
}

output "subnet_name" {
  description = "Private subnet name."
  value       = google_compute_subnetwork.private.name
}

output "node_service_account" {
  description = "Email of the node pool's service account."
  value       = google_service_account.nodes.email
}
