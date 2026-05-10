locals {
  environment  = "prod"
  cluster_name = "kubewatcher-prod"

  tags = {
    Project     = "kubewatcher"
    Environment = local.environment
    ManagedBy   = "terraform"
    Stack       = "live/prod/aws"
  }
}

module "platform" {
  source = "../../../modules/eks-platform"

  environment  = local.environment
  cluster_name = local.cluster_name
  tags         = local.tags

  cluster_version                = "1.30"
  cluster_endpoint_public_access = false # prod API is private; reach via VPN / SSM tunnel

  availability_zones   = ["us-west-2a", "us-west-2b", "us-west-2c"]
  private_subnet_cidrs = ["10.20.1.0/24", "10.20.2.0/24", "10.20.3.0/24"]
  public_subnet_cidrs  = ["10.20.101.0/24", "10.20.102.0/24", "10.20.103.0/24"]
  vpc_cidr             = "10.20.0.0/16"

  node_group_instance_types = ["m5.large"]
  node_group_desired_size   = 4
  node_group_min_size       = 3
  node_group_max_size       = 12

  enable_night_scale_in = false
}

module "apps" {
  source = "../../../modules/kubewatcher-helm"

  environment = local.environment
  chart_path  = "${path.module}/../../../../deploy/helm"

  image_tag = var.image_tag

  backend_enabled       = true
  frontend_enabled      = true
  monitoring_enabled    = true
  agent_enabled         = true
  alert_ingress_enabled = true
  mcp_enabled           = true

  argocd_url    = var.argocd_url
  anomstack_url = var.anomstack_url

  deepseek_api_key       = var.deepseek_api_key
  argocd_token           = var.argocd_token
  grafana_admin_password = var.grafana_admin_password

  depends_on = [module.platform]
}
