output "cluster_name" {
  description = "DOKS cluster name."
  value       = digitalocean_kubernetes_cluster.primary.name
}

output "cluster_endpoint" {
  description = "DOKS cluster API endpoint."
  value       = digitalocean_kubernetes_cluster.primary.endpoint
}

output "cluster_ca_certificate" {
  description = "Base64-encoded CA certificate for the cluster API."
  value       = digitalocean_kubernetes_cluster.primary.kube_config[0].cluster_ca_certificate
}

output "cluster_token" {
  description = "DOKS cluster bearer token."
  value       = digitalocean_kubernetes_cluster.primary.kube_config[0].token
  sensitive   = true
}

output "cluster_id" {
  description = "DOKS cluster ID."
  value       = digitalocean_kubernetes_cluster.primary.id
}

output "vpc_id" {
  description = "VPC ID the cluster lives in."
  value       = digitalocean_vpc.cluster.id
}

output "ipv4_address" {
  description = "Cluster public IPv4 address."
  value       = digitalocean_kubernetes_cluster.primary.ipv4_address
}
