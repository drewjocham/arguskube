variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-west1"
}

variable "name_prefix" {
  description = "Name prefix for all resources"
  type        = string
  default     = "kubewatcher"
}

# ─── Global Config ───────────────────────────────────────────────────

variable "tier" {
  description = "KubeWatcher feature tier"
  type        = string
  default     = "pro"
}

variable "log_level" {
  description = "Log level"
  type        = string
  default     = "info"
}

variable "log_format" {
  description = "Log format (json or text)"
  type        = string
  default     = "json"
}

variable "service_account_email" {
  description = "Service account for Cloud Run services"
  type        = string
}

variable "vpc_connector" {
  description = "VPC access connector for Cloud Run"
  type        = string
  default     = ""
}

variable "cloudsql_instance_name" {
  description = "Cloud SQL instance name (if using PG instead of local SQLite)"
  type        = string
  default     = ""
}

# ─── Images ──────────────────────────────────────────────────────────

variable "backend_image" {
  description = "Backend container image"
  type        = string
}

variable "frontend_image" {
  description = "Frontend container image"
  type        = string
}

variable "mcp_image" {
  description = "MCP container image"
  type        = string
}

# ─── Backend Config ──────────────────────────────────────────────────

variable "backend_port" {
  description = "Backend HTTP port"
  type        = string
  default     = "8080"
}

variable "backend_cpu" {
  description = "Backend CPU limit"
  type        = string
  default     = "2"
}

variable "backend_memory" {
  description = "Backend memory limit"
  type        = string
  default     = "1Gi"
}

variable "backend_min_instances" {
  description = "Backend min instances"
  type        = string
  default     = "1"
}

variable "backend_max_instances" {
  description = "Backend max instances"
  type        = string
  default     = "10"
}

# ─── Frontend Config ─────────────────────────────────────────────────

variable "frontend_cpu" {
  description = "Frontend CPU limit"
  type        = string
  default     = "1"
}

variable "frontend_memory" {
  description = "Frontend memory limit"
  type        = string
  default     = "512Mi"
}

variable "frontend_min_instances" {
  description = "Frontend min instances"
  type        = string
  default     = "1"
}

variable "frontend_max_instances" {
  description = "Frontend max instances"
  type        = string
  default     = "5"
}

# ─── MCP Config ──────────────────────────────────────────────────────

variable "mcp_cpu" {
  description = "MCP CPU limit"
  type        = string
  default     = "1"
}

variable "mcp_memory" {
  description = "MCP memory limit"
  type        = string
  default     = "512Mi"
}

variable "mcp_min_instances" {
  description = "MCP min instances"
  type        = string
  default     = "1"
}

variable "mcp_max_instances" {
  description = "MCP max instances"
  type        = string
  default     = "5"
}

# ─── External Service URLs ───────────────────────────────────────────

variable "flink_gateway_url" {
  description = "URL of the Flink anomaly detection gateway"
  type        = string
}

variable "prometheus_url" {
  description = "Prometheus server URL"
  type        = string
  default     = ""
}

# ─── Secrets ─────────────────────────────────────────────────────────

variable "flink_api_key_secret" {
  description = "Name of the Secret Manager secret for Flink API key"
  type        = string
  default     = ""
}

variable "deepseek_api_key_secret" {
  description = "Name of the Secret Manager secret for DeepSeek API key"
  type        = string
  default     = ""
}
