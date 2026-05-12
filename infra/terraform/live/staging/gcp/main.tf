locals {
  environment  = "staging"
  cluster_name = "kubewatcher-staging"

  tags = {
    Project     = "kubewatcher"
    Environment = local.environment
    ManagedBy   = "terraform"
    Stack       = "live/staging/gcp"
  }
}

module "platform" {
  source = "../../../modules/gke-platform"

  project_id   = var.project_id
  environment  = local.environment
  cluster_name = local.cluster_name
  tags         = local.tags
  region       = var.region

  enable_private_endpoint = true

  spot            = false
  machine_type    = "e2-standard-2"
  node_count_min  = 1
  node_count_max  = 5
  disk_size_gb    = 50

  release_channel = "REGULAR"
}

module "apps" {
  source = "../../../modules/kubewatcher-helm"

  environment = local.environment
  chart_path  = "${path.module}/../../../../charts"

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
