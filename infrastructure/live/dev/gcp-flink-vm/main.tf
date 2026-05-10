module "flink" {
  source = "../../../modules/flink-vm"

  project_id = var.project_id
  region     = var.region
  zone       = var.zone

  name_prefix = "kubewatcher-dev"

  network    = var.network
  subnetwork = var.subnetwork

  machine_type     = "e2-standard-2" # smaller for dev
  disk_size_gb     = 50
  assign_public_ip = false

  gateway_api_key         = var.gateway_api_key
  kafka_bootstrap_servers = var.kafka_bootstrap_servers
}
