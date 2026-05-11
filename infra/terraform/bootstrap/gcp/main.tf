# One-time bootstrap: create the GCS bucket that every
# `live/<env>/aws` workspace uses for remote state.
#
# GCS provides strong consistency natively — no lock table needed.
#
# Run this exactly once per GCP project. Apply with LOCAL state:
#
#   cd infra/terraform/bootstrap/gcp
#   gcloud auth application-default login
#   terraform init
#   terraform apply -var project_id=my-gcp-project
#
# After apply, migrate state into the bucket:
#   uncomment the backend block below, then terraform init -migrate-state

terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }

  # After apply, uncomment this and run terraform init -migrate-state:
  # backend "gcs" {
  #   bucket = "kubewatcher-tfstate"
  #   prefix = "bootstrap/gcp"
  # }
}

variable "project_id" {
  description = "GCP project ID for the state bucket."
  type        = string
}

variable "state_bucket_name" {
  description = "Globally-unique GCS bucket name for Terraform state."
  type        = string
  default     = "kubewatcher-tfstate"
}

variable "region" {
  description = "GCS bucket location."
  type        = string
  default     = "US"
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# ── State bucket ──────────────────────────────────────────────────
resource "google_storage_bucket" "tfstate" {
  name          = var.state_bucket_name
  location      = var.region
  force_destroy = false

  versioning {
    enabled = true
  }

  uniform_bucket_level_access = true

  labels = {
    project    = "kubewatcher"
    stack      = "bootstrap-gcp"
    managed-by = "terraform"
  }
}

resource "google_storage_bucket_iam_binding" "tfstate_admins" {
  bucket = google_storage_bucket.tfstate.name
  role   = "roles/storage.objectAdmin"

  members = [
    "projectOwner:${var.project_id}",
  ]
}

output "state_bucket" {
  description = "Use this in every live/<env>/aws/backend.tf as `bucket`."
  value       = google_storage_bucket.tfstate.name
}

output "region" {
  value = var.region
}
