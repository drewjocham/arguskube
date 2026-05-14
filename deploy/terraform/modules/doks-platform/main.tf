# ── DigitalOcean VPC ──────────────────────────────────────────────
resource "digitalocean_vpc" "cluster" {
  name     = "${var.cluster_name}-vpc"
  region   = var.region
  ip_range = "10.10.0.0/16"
}

# ── DOKS Cluster ─────────────────────────────────────────────────
resource "digitalocean_kubernetes_cluster" "primary" {
  name    = var.cluster_name
  region  = var.region
  version = var.kubernetes_version

  vpc_uuid = digitalocean_vpc.cluster.id

  # Default node pool — DigitalOcean requires at least one
  node_pool {
    name       = "${var.cluster_name}-pool"
    size       = var.node_size
    min_nodes  = var.node_count_min
    max_nodes  = var.node_count_max

    auto_scale = true

    tags = var.tags

    lifecycle {
      ignore_changes = [
        node_count,
      ]
    }
  }

  maintenance_policy {
    day        = "sunday"
    start_time = "04:00"
  }

  # Surge upgrades — faster but temporarily runs extra droplets
  surge_upgrade = var.enable_surge_upgrade

  tags = var.tags
}
