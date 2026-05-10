variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "GCP region for the Cloud Run services."
  type        = string
  default     = "us-west1"
}

variable "name_prefix" {
  description = "Prefix for every resource name (services, secrets)."
  type        = string
  default     = "kubewatcher"
}

# ── Global config ─────────────────────────────────────────────────
variable "tier"       { type = string; default = "pro" }
variable "log_level"  { type = string; default = "info" }
variable "log_format" { type = string; default = "json" }

variable "service_account_email" {
  description = "Service account attached to every Cloud Run service in this module."
  type        = string
}

variable "vpc_connector" {
  description = "VPC access connector — required for the backend to reach Cloud SQL or a private Flink VM."
  type        = string
  default     = ""
}

variable "cloudsql_instance_name" {
  description = "Cloud SQL instance to attach to the backend (project:region:instance form)."
  type        = string
  default     = ""
}

# ── Images ────────────────────────────────────────────────────────
variable "backend_image"  { type = string }
variable "frontend_image" { type = string }
variable "mcp_image"      { type = string }

# ── Backend sizing ────────────────────────────────────────────────
variable "backend_port"          { type = string; default = "8080" }
variable "backend_cpu"            { type = string; default = "2" }
variable "backend_memory"         { type = string; default = "1Gi" }
variable "backend_min_instances"  { type = string; default = "1" }
variable "backend_max_instances"  { type = string; default = "10" }

# ── Frontend sizing ───────────────────────────────────────────────
variable "frontend_cpu"            { type = string; default = "1" }
variable "frontend_memory"         { type = string; default = "512Mi" }
variable "frontend_min_instances"  { type = string; default = "1" }
variable "frontend_max_instances"  { type = string; default = "5" }

# ── MCP sizing ────────────────────────────────────────────────────
variable "mcp_cpu"            { type = string; default = "1" }
variable "mcp_memory"         { type = string; default = "512Mi" }
variable "mcp_min_instances"  { type = string; default = "1" }
variable "mcp_max_instances"  { type = string; default = "5" }

# ── External services ─────────────────────────────────────────────
variable "flink_gateway_url" {
  description = "URL of the Flink anomaly-detection gateway."
  type        = string
}

variable "prometheus_url" {
  description = "Prometheus URL the backend scrapes for metrics."
  type        = string
  default     = ""
}
