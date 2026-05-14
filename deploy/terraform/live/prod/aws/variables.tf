# Per-environment knobs. Defaults baked in here are dev-friendly;
# secrets are passed via *.auto.tfvars (gitignored) or CI env vars.

variable "aws_region" {
  description = "AWS region for the dev cluster."
  type        = string
  default     = "us-west-2"
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
