# GCP equivalent of bootstrap/aws — creates the GCS bucket every
# `live/<env>/gcp-*` workspace uses for remote state.
#
# GCS object versioning + native object locks make this much simpler
# than the AWS path: no separate lock table, no DynamoDB, no KMS
# choreography. Bucket-level retention + uniform IAM is enough.

terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

resource "google_storage_bucket" "tfstate" {
  name                        = var.state_bucket_name
  location                    = var.region
  force_destroy               = false
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      num_newer_versions = 10
    }
    action {
      type = "Delete"
    }
  }

  labels = {
    project = "kubewatcher"
    stack   = "bootstrap-gcp"
  }
}
