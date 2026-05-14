locals {
  environment  = "dev"
  cluster_name = "kubewatcher-dev"

  tags = ["kubewatcher", "dev"]
}

module "platform" {
  source = "../../../modules/doks-platform"

  environment      = local.environment
  cluster_name     = local.cluster_name
  region           = var.region
  tags             = local.tags

  node_size        = "s-2vcpu-2gb"
  node_count_min   = 0
  node_count_max   = 3
  node_count_initial = 1
}

module "apps" {
  source = "../../../modules/kubewatcher-helm"

  environment = local.environment
  chart_path  = "${path.module}/../../../../charts"

  image_tag = var.image_tag

  backend_enabled       = true
  frontend_enabled      = true
  monitoring_enabled    = true
  agent_enabled         = false
  alert_ingress_enabled = true
  mcp_enabled           = false

  argocd_url    = var.argocd_url
  anomstack_url = var.anomstack_url

  deepseek_api_key       = var.deepseek_api_key
  argocd_token           = var.argocd_token
  grafana_admin_password = var.grafana_admin_password

  depends_on = [module.platform]
}
