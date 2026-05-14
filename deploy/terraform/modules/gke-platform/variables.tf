variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "environment" {
  description = "Deployment environment (dev, staging, prod)."
  type        = string

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

variable "cluster_name" {
  description = "GKE cluster name."
  type        = string
}

variable "region" {
  description = "GCP region for the cluster."
  type        = string
  default     = "us-west1"
}

variable "tags" {
  description = "Labels applied to every resource."
  type        = map(string)
  default     = {}
}

variable "enable_private_endpoint" {
  description = "Restrict the cluster API to the VPC. False for dev."
  type        = bool
  default     = false
}

variable "release_channel" {
  description = "GKE release channel: UNSPECIFIED, RAPID, REGULAR, STABLE."
  type        = string
  default     = "REGULAR"
}

# ── Cost: Spot + machine type ────────────────────────────────────
variable "spot" {
  description = "Use preemptible (spot) nodes — ~60-80% cheaper, can be reclaimed anytime."
  type        = bool
  default     = true
}

variable "machine_type" {
  description = "GCE machine type for nodes. For spot nodes, e2-small or e2-medium keep costs under ~$20/mo."
  type        = string
  default     = "e2-small"
}

variable "node_count_min" {
  description = "Minimum node count. Set to 0 for true scale-to-zero when idle."
  type        = number
  default     = 0
}

variable "node_count_max" {
  description = "Maximum node count for autoscaling."
  type        = number
  default     = 3
}

variable "disk_size_gb" {
  description = "Node boot disk size in GB."
  type        = number
  default     = 30
}

# ── Sleep schedule (hard scale-to-zero on a cron) ─────────────────
# When enabled, creates a Cloud Function + Cloud Scheduler that sets
# node pool min=0 at night and restores it in the morning.
variable "enable_sleep_schedule" {
  description = "Create Cloud Scheduler + Cloud Function to force scale node pool to 0 on a schedule."
  type        = bool
  default     = false
}

variable "sleep_cron" {
  description = "Cron (UTC) for scaling node pool to 0."
  type        = string
  default     = "0 5 * * *"
}

variable "wake_cron" {
  description = "Cron (UTC) for restoring node pool min size."
  type        = string
  default     = "0 13 * * 1-5"
}

variable "scaling_time_zone" {
  description = "IANA time zone for the crons."
  type        = string
  default     = "America/Los_Angeles"
}
