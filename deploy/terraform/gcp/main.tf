terraform {
  required_version = ">= 1.6"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.40"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

# Static external IP — keeps the endpoint URL stable across recreates.
resource "google_compute_address" "llm" {
  name   = "${var.name}-ip"
  region = var.region
}

# Allow inbound traffic to the vLLM port and SSH.
resource "google_compute_firewall" "llm_ingress" {
  name    = "${var.name}-allow"
  network = var.network

  allow {
    protocol = "tcp"
    ports    = [tostring(var.port), "22"]
  }

  source_ranges = var.allowed_source_ranges
  target_tags   = ["llm-server"]
}

# Bootstrap script — same one vast.ai uses, sourced from the repo.
locals {
  startup_script = <<-EOT
    #!/usr/bin/env bash
    set -euxo pipefail
    export MODEL_NAME=${var.model_name}
    export LLM_API_KEY=${var.llm_api_key}
    export PORT=${var.port}
    export GPU_MEM_FRAC=${var.gpu_mem_fraction}
    %{if var.hf_token != ""}export HF_TOKEN=${var.hf_token}%{endif}
    ${file("${path.module}/../cloud-init/llm-server.sh")}
  EOT
}

resource "google_compute_instance" "llm" {
  name         = var.name
  machine_type = var.machine_type
  zone         = var.zone
  tags         = ["llm-server"]

  # Spot/preemptible by default — much cheaper, can be reclaimed by GCP.
  # Set var.preemptible = false for an on-demand SLA.
  scheduling {
    preemptible                 = var.preemptible
    automatic_restart           = !var.preemptible
    provisioning_model          = var.preemptible ? "SPOT" : "STANDARD"
    instance_termination_action = var.preemptible ? "STOP" : null
    on_host_maintenance         = "TERMINATE" # required for GPU instances
  }

  guest_accelerator {
    type  = var.gpu_type
    count = var.gpu_count
  }

  boot_disk {
    initialize_params {
      # NVIDIA Deep Learning VM image — has CUDA + drivers preinstalled so the
      # cloud-init script only has to install Docker + the toolkit.
      image = var.boot_image
      size  = var.disk_gb
      type  = "pd-balanced"
    }
  }

  network_interface {
    network    = var.network
    subnetwork = var.subnetwork

    access_config {
      nat_ip = google_compute_address.llm.address
    }
  }

  metadata = {
    # NVIDIA driver auto-install (DLVM images respect this).
    install-nvidia-driver = "True"
  }

  metadata_startup_script = local.startup_script

  service_account {
    email  = var.service_account_email
    scopes = var.service_account_scopes
  }

  # Allow Terraform to recreate cleanly when the startup script changes.
  lifecycle {
    create_before_destroy = false
  }
}
