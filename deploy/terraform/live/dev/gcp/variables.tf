variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "GCP region."
  type        = string
  default     = "us-west1"
}

variable "image_tag" {
  description = "Override container image tag. Empty falls back to chart appVersion."
  type        = string
  default     = ""
}

variable "deepseek_api_key" {
  description = "DeepSeek API key for backend AI features."
  type        = string
  sensitive   = true
  default     = ""
}

variable "argocd_url" {
  description = "Argo CD URL the backend integrates with."
  type        = string
  default     = ""
}

variable "argocd_token" {
  description = "Argo CD API token."
  type        = string
  sensitive   = true
  default     = ""
}

variable "anomstack_url" {
  description = "Anomstack service URL."
  type        = string
  default     = ""
}

variable "grafana_admin_password" {
  description = "Grafana admin password — required when monitoring is enabled."
  type        = string
  sensitive   = true
  default     = ""
}
