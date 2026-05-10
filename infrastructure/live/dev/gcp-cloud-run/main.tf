module "cloud_run" {
  source = "../../../modules/cloud-run"

  project_id = var.project_id
  region     = var.region

  name_prefix = "kubewatcher-dev"
  tier        = "free"

  service_account_email = var.service_account_email
  vpc_connector         = var.vpc_connector

  backend_image  = var.backend_image
  frontend_image = var.frontend_image
  mcp_image      = var.mcp_image

  # Dev sizing — small instances, scale to zero allowed.
  backend_min_instances  = "0"
  frontend_min_instances = "0"
  mcp_min_instances      = "0"

  flink_gateway_url = var.flink_gateway_url
  prometheus_url    = var.prometheus_url
}
