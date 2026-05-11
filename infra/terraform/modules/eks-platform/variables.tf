variable "environment" {
  description = "Deployment environment (dev, staging, prod). Drives sizing and dev-only behaviors."
  type        = string

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

variable "tags" {
  description = "Tags applied to every resource the module creates."
  type        = map(string)
  default     = {}
}

# ── VPC ───────────────────────────────────────────────────────────
variable "vpc_cidr" {
  description = "CIDR block for the cluster VPC."
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "AZs for cluster subnets."
  type        = list(string)
}

variable "private_subnet_cidrs" {
  description = "CIDRs for the private subnets that host the node group."
  type        = list(string)
}

variable "public_subnet_cidrs" {
  description = "CIDRs for the public-facing subnets used by load balancers."
  type        = list(string)
}

# ── EKS ───────────────────────────────────────────────────────────
variable "cluster_name" {
  description = "Name of the EKS cluster."
  type        = string
}

variable "cluster_version" {
  description = "Kubernetes minor version, e.g. \"1.30\"."
  type        = string
  default     = "1.30"
}

variable "cluster_endpoint_public_access" {
  description = "Expose the cluster API publicly."
  type        = bool
  default     = false
}

# ── Node group ────────────────────────────────────────────────────
variable "node_group_instance_types" {
  description = "Instance types for the managed node group."
  type        = list(string)
  default     = ["t3.medium"]
}

variable "node_group_desired_size" {
  description = "Desired node count outside scheduled scaling."
  type        = number
  default     = 2
}

variable "node_group_min_size" {
  description = "Min node count. 0 enables scale-to-zero."
  type        = number
  default     = 1
}

variable "node_group_max_size" {
  description = "Cap for autoscaling."
  type        = number
  default     = 6
}

# ── Scheduled scaling (dev only) ──────────────────────────────────
variable "use_spot" {
  description = "Use spot instances (~60% cheaper, can be reclaimed)."
  type        = bool
  default     = true
}

variable "enable_night_scale_in" {
  description = "Scale node group to 0 outside business hours."
  type        = bool
  default     = false
}

variable "scale_in_cron" {
  description = "Cron expression for scaling to zero (UTC)."
  type        = string
  default     = "0 5 * * *"
}

variable "scale_out_cron" {
  description = "Cron expression for scaling back up."
  type        = string
  default     = "0 13 * * 1-5"
}

variable "scaling_time_zone" {
  description = "IANA time zone for the scaling crons."
  type        = string
  default     = "America/Los_Angeles"
}
