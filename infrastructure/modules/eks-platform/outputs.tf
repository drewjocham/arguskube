output "cluster_name" {
  description = "EKS cluster name."
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS API endpoint."
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority_data" {
  description = "Base64-encoded CA bundle for the EKS API."
  value       = module.eks.cluster_certificate_authority_data
}

output "cluster_security_group_id" {
  description = "Security group used by the EKS control plane."
  value       = module.eks.cluster_security_group_id
}

output "oidc_provider_arn" {
  description = "OIDC provider ARN — used to bind IAM roles to service accounts."
  value       = module.eks.oidc_provider_arn
}

output "vpc_id" {
  description = "VPC the cluster lives in."
  value       = module.vpc.vpc_id
}

output "private_subnet_ids" {
  description = "Private subnet IDs (where the node group runs)."
  value       = module.vpc.private_subnets
}

output "public_subnet_ids" {
  description = "Public subnet IDs (for ELB ingress)."
  value       = module.vpc.public_subnets
}

output "node_group" {
  description = "EKS managed node group object — exposed for autoscaling-group lookups."
  value       = module.eks.eks_managed_node_groups["main"]
}
