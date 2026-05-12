output "cluster_name" {
  description = "EKS cluster name"
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS cluster endpoint"
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority_data" {
  description = "EKS cluster certificate authority data"
  value       = module.eks.cluster_certificate_authority_data
}

output "cluster_security_group_id" {
  description = "Security group ID for the EKS cluster"
  value       = module.eks.cluster_security_group_id
}

output "oidc_provider_arn" {
  description = "OIDC provider ARN for IAM roles"
  value       = module.eks.oidc_provider_arn
}

output "node_group_arn" {
  description = "Node group ARN"
  value       = module.eks.eks_managed_node_groups["main"]
}

output "namespace" {
  description = "Kubernetes namespace for KubeWatcher"
  value       = "kubewatcher"
}

output "backend_service" {
  description = "Backend service name"
  value       = "kubewatcher-backend"
}

output "frontend_service" {
  description = "Frontend service name"
  value       = "kubewatcher-frontend"
}

output "vpc_id" {
  description = "VPC ID"
  value       = module.vpc.vpc_id
}

output "private_subnet_ids" {
  description = "Private subnet IDs"
  value       = module.vpc.private_subnets
}

output "public_subnet_ids" {
  description = "Public subnet IDs"
  value       = module.vpc.public_subnets
}
