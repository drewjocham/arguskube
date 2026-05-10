variable "aws_region" {
  description = "Region the state bucket + lock table live in. Pick once and stick to it."
  type        = string
  default     = "us-east-1"
}

variable "state_bucket_name" {
  description = "Globally-unique S3 bucket name for Terraform state."
  type        = string
  default     = "kubewatcher-tfstate"
}

variable "lock_table_name" {
  description = "DynamoDB table name for state locking."
  type        = string
  default     = "kubewatcher-tfstate-lock"
}
