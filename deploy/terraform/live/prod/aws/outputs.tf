output "cluster_name"     { value = module.platform.cluster_name }
output "cluster_endpoint" { value = module.platform.cluster_endpoint }
output "namespace"        { value = module.apps.namespace }

output "kubeconfig_command" {
  description = "Copy-paste this to point kubectl at the cluster (requires AWS auth + VPN if private endpoint)."
  value       = "aws eks update-kubeconfig --region ${var.aws_region} --name ${module.platform.cluster_name}"
}
