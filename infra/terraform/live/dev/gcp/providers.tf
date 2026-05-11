terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.25"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region

  default_labels = local.tags
}

data "google_client_config" "default" {}

provider "kubernetes" {
  host                   = "https://${module.platform.cluster_endpoint}"
  cluster_ca_certificate = base64decode(module.platform.cluster_ca_certificate)
  token                  = data.google_client_config.default.access_token
}

provider "helm" {
  kubernetes {
    host                   = "https://${module.platform.cluster_endpoint}"
    cluster_ca_certificate = base64decode(module.platform.cluster_ca_certificate)
    token                  = data.google_client_config.default.access_token
  }
}
