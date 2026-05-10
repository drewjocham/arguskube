# Flink VM module — provisions a single Compute Engine instance running
# Apache Flink for the KubeWatcher anomaly-detection backend.
#
# Provider configuration is the caller's responsibility (root module).
# This module is intentionally provider-config-free so it can be used
# from multi-project compositions without provider aliasing.

resource "google_compute_instance" "flink" {
  name         = "${var.name_prefix}-flink"
  machine_type = var.machine_type
  zone         = var.zone

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
      size  = var.disk_size_gb
    }
  }

  network_interface {
    network    = var.network
    subnetwork = var.subnetwork

    access_config {
      nat_ip = var.assign_public_ip ? google_compute_address.flink[0].address : null
    }
  }

  metadata = {
    flink-version    = var.flink_version
    kubewatcher-role = "anomaly-detection"
  }

  metadata_startup_script = templatefile("${path.module}/startup.sh", {
    flink_version = var.flink_version
    gateway_port  = var.gateway_port
    api_key       = var.gateway_api_key
    kafka_servers = var.kafka_bootstrap_servers
  })

  service_account {
    email  = var.service_account_email != null ? var.service_account_email : google_service_account.flink_sa.email
    scopes = ["cloud-platform"]
  }

  tags = ["flink", "kubewatcher"]

  depends_on = [google_service_account.flink_sa]
}

resource "google_compute_address" "flink" {
  count  = var.assign_public_ip ? 1 : 0
  name   = "${var.name_prefix}-flink-ip"
  region = var.region
}

resource "google_compute_firewall" "flink_allow_internal" {
  name    = "${var.name_prefix}-flink-allow-internal"
  network = var.network

  allow {
    protocol = "tcp"
    ports    = ["8081", var.gateway_port, "6123"]
  }

  source_ranges = [var.internal_cidr]
  target_tags   = ["flink"]
}

resource "google_compute_firewall" "flink_allow_gateway" {
  count   = var.allow_gateway_ingress ? 1 : 0
  name    = "${var.name_prefix}-flink-allow-gateway"
  network = var.network

  allow {
    protocol = "tcp"
    ports    = [var.gateway_port]
  }

  source_ranges = var.gateway_allowed_cidrs
  target_tags   = ["flink"]
}

resource "google_service_account" "flink_sa" {
  account_id   = "${var.name_prefix}-flink-sa"
  display_name = "Flink VM Service Account"
}

resource "google_project_iam_member" "flink_metrics" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.flink_sa.email}"
}
