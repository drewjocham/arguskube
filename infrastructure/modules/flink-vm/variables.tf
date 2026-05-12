variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "GCP region for regional resources (public IP, firewalls)."
  type        = string
  default     = "us-west1"
}

variable "zone" {
  description = "GCP zone for the instance."
  type        = string
  default     = "us-west1-a"
}

variable "name_prefix" {
  description = "Prefix applied to every resource name."
  type        = string
  default     = "argus"
}

variable "machine_type" {
  description = "GCE machine type."
  type        = string
  default     = "e2-standard-4"
}

variable "disk_size_gb" {
  description = "Boot disk size in GB."
  type        = number
  default     = 100
}

# ── Networking ────────────────────────────────────────────────────
variable "network" {
  description = "VPC network self-link or name."
  type        = string
}

variable "subnetwork" {
  description = "VPC subnetwork self-link or name."
  type        = string
}

variable "internal_cidr" {
  description = "CIDR for internal-only firewall ingress (typically the VPC range)."
  type        = string
  default     = "10.0.0.0/16"
}

variable "assign_public_ip" {
  description = "Allocate a public IP for the instance."
  type        = bool
  default     = false
}

variable "allow_gateway_ingress" {
  description = "Open the gateway port to gateway_allowed_cidrs."
  type        = bool
  default     = false
}

variable "gateway_allowed_cidrs" {
  description = "Source CIDRs allowed to reach the gateway port."
  type        = list(string)
  default     = []
}

# ── Flink ─────────────────────────────────────────────────────────
variable "flink_version" {
  description = "Apache Flink version to deploy."
  type        = string
  default     = "1.18.0"
}

variable "gateway_port" {
  description = "Port where the anomaly-detection gateway listens."
  type        = string
  default     = "8087"
}

variable "gateway_api_key" {
  description = "API key for the gateway. Stored in instance metadata; rotate via tfvars."
  type        = string
  sensitive   = true
}

variable "service_account_email" {
  description = "Service account to attach to the instance. null = use the SA this module creates."
  type        = string
  default     = null
}

variable "kafka_bootstrap_servers" {
  description = "Kafka bootstrap servers Flink ingests metrics from."
  type        = string
  default     = "localhost:9092"
}
