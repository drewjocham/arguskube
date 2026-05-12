variable "environment" {
  description = "Deployment environment (dev, staging, prod)."
  type        = string

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

variable "cluster_name" {
  description = "DOKS cluster name."
  type        = string
}

variable "region" {
  description = "DigitalOcean region slug, e.g. nyc1, sfo3, ams3."
  type        = string
  default     = "sfo3"
}

variable "tags" {
  description = "Tags applied to every resource."
  type        = list(string)
  default     = ["kubewatcher"]
}

variable "kubernetes_version" {
  description = "Kubernetes version (empty = DO latest)."
  type        = string
  default     = ""
}

variable "node_size" {
  description = "Droplet size slug for nodes, e.g. s-2vcpu-2gb, g-4vcpu-8gb."
  type        = string
  default     = "s-2vcpu-2gb"
}

variable "node_count_min" {
  description = "Minimum node count."
  type        = number
  default     = 1
}

variable "node_count_max" {
  description = "Maximum node count."
  type        = number
  default     = 3
}

variable "node_count_initial" {
  description = "Initial node count."
  type        = number
  default     = 1
}

variable "enable_auto_upgrade" {
  description = "Enable automatic Kubernetes patch upgrades."
  type        = bool
  default     = true
}

variable "enable_surge_upgrade" {
  description = "Enable surge upgrades (replaces nodes with new ones)."
  type        = bool
  default     = true
}

variable "max_surge_nodes" {
  description = "Max nodes created during surge upgrade."
  type        = number
  default     = 1
}
