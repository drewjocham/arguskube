# One-time bootstrap: create the S3 bucket + DynamoDB table that every
# `live/<env>/aws` workspace uses for remote state and locking.
#
# Run this exactly once per AWS account. Apply with LOCAL state:
#
#   cd infrastructure/bootstrap/aws
#   terraform init
#   terraform apply
#
# After apply, commit the resulting `terraform.tfstate` to a secrets
# vault — losing it means losing the ability to manage these
# foundational resources, but they're trivial to recreate (and
# re-import) if needed.

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = "argus"
      Stack     = "bootstrap/aws"
      ManagedBy = "terraform"
    }
  }
}

# ── State bucket ──────────────────────────────────────────────────
resource "aws_s3_bucket" "tfstate" {
  bucket = var.state_bucket_name
}

resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "aws:kms"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# ── Lock table ────────────────────────────────────────────────────
# Pay-per-request — locks are infrequent enough that provisioned
# capacity would be wasted spend.
resource "aws_dynamodb_table" "tfstate_lock" {
  name         = var.lock_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }
}
