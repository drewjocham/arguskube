output "cluster_name" {
  description = "GKE cluster name."
  value       = google_container_cluster.runner.name
}

output "cluster_endpoint" {
  description = "GKE cluster API endpoint."
  value       = google_container_cluster.runner.endpoint
}

output "cluster_ca_certificate" {
  description = "Base64-encoded CA certificate for the cluster API."
  value       = google_container_cluster.runner.master_auth[0].cluster_ca_certificate
}

output "broker_endpoint" {
  description = "In-cluster broker endpoint (service name)."
  value       = local.broker_endpoint
}

output "broker_namespace" {
  description = "Namespace where the broker was installed."
  value       = kubernetes_namespace.broker.metadata[0].name
}

output "kubeconfig" {
  description = "Kubeconfig content for connecting to this cluster."
  value = templatefile("${path.module}/templates/kubeconfig.tftpl", {
    cluster_name    = google_container_cluster.runner.name
    cluster_endpoint = "https://${google_container_cluster.runner.endpoint}"
    cluster_ca      = google_container_cluster.runner.master_auth[0].cluster_ca_certificate
  })
  sensitive = true
}
