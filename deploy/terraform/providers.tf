terraform {
  required_version = ">= 1.5"

  # ── Remote state ────────────────────────────────────────────────────
  # State must NOT live alongside this code in source control once you
  # apply against any shared environment — losing it means losing track
  # of every resource Terraform created.
  #
  # Bootstrap once (run as a privileged user):
  #
  #   aws s3api create-bucket \
  #     --bucket kubewatcher-tfstate-<account> \
  #     --region us-east-1
  #   aws s3api put-bucket-versioning \
  #     --bucket kubewatcher-tfstate-<account> \
  #     --versioning-configuration Status=Enabled
  #   aws s3api put-bucket-encryption \
  #     --bucket kubewatcher-tfstate-<account> \
  #     --server-side-encryption-configuration \
  #       '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"aws:kms"}}]}'
  #   aws dynamodb create-table \
  #     --table-name kubewatcher-tfstate-lock \
  #     --attribute-definitions AttributeName=LockID,AttributeType=S \
  #     --key-schema AttributeName=LockID,KeyType=HASH \
  #     --billing-mode PAY_PER_REQUEST
  #
  # Then uncomment the block below and `terraform init -migrate-state`.
  #
  # backend "s3" {
  #   bucket         = "kubewatcher-tfstate-<account>"
  #   key            = "kubewatcher/terraform.tfstate"
  #   region         = "us-east-1"
  #   encrypt        = true
  #   kms_key_id     = "alias/aws/s3"
  #   dynamodb_table = "kubewatcher-tfstate-lock"
  # }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.25"
    }
    kubectl = {
      source  = "gavinbunney/kubectl"
      version = "~> 1.14"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

# ── AWS Provider ──────────────────────────────────────────────────
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = var.tags
  }
}

# ── Kubernetes Provider (EKS) ─────────────────────────────────────
data "aws_eks_cluster" "main" {
  name = module.eks.cluster_name
}

data "aws_eks_cluster_auth" "main" {
  name = module.eks.cluster_name
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.main.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.main.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.main.token
}

provider "helm" {
  kubernetes {
    host                   = data.aws_eks_cluster.main.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.main.certificate_authority[0].data)
    token                  = data.aws_eks_cluster_auth.main.token
  }
}

provider "kubectl" {
  host                   = data.aws_eks_cluster.main.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.main.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.main.token
  load_config_file       = false
}
