variable "project_id" {
  description = "GCP project ID hosting the state bucket."
  type        = string
}

variable "region" {
  description = "GCS bucket location. Pick a multi-region (e.g. \"US\") or a single region."
  type        = string
  default     = "US"
}

variable "state_bucket_name" {
  description = "Globally-unique GCS bucket name for Terraform state."
  type        = string
  default     = "kubewatcher-tfstate"
}
