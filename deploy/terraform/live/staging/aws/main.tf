locals {
  environment  = "staging"
  cluster_name = "argus-staging"

  tags = {
    Project     = "argus"
    Environment = local.environment
    ManagedBy   = "terraform"
    Stack       = "live/staging/aws"
  }
}

module "platform" {
  source = "../../../modules/eks-platform"

  environment  = local.environment
  cluster_name = local.cluster_name
  tags         = local.tags

  cluster_version                = "1.30"
  cluster_endpoint_public_access = false # staging mirrors prod's network posture

  availability_zones   = ["us-west-2a", "us-west-2b", "us-west-2c"]
  private_subnet_cidrs = ["10.10.1.0/24", "10.10.2.0/24", "10.10.3.0/24"]
  public_subnet_cidrs  = ["10.10.101.0/24", "10.10.102.0/24", "10.10.103.0/24"]
  vpc_cidr             = "10.10.0.0/16"

  node_group_instance_types = ["t3.large"]
  node_group_desired_size   = 3
  node_group_min_size       = 2
  node_group_max_size       = 8

  enable_night_scale_in = false
}

module "apps" {
  source = "../../../modules/argus-helm"

  environment = local.environment
  chart_path  = "${path.module}/../../../../deploy/helm"

  image_tag = var.image_tag

  backend_enabled       = true
  frontend_enabled      = true
  monitoring_enabled    = true
  agent_enabled         = true
  alert_ingress_enabled = true
  mcp_enabled           = false

  argocd_url    = var.argocd_url
  anomstack_url = var.anomstack_url

  deepseek_api_key       = var.deepseek_api_key
  argocd_token           = var.argocd_token
  grafana_admin_password = var.grafana_admin_password

  depends_on = [module.platform]
}
