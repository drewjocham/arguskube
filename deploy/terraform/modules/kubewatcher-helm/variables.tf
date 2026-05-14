variable "environment" {
  description = "Deployment environment (dev, staging, prod). Drives sizing and resource hints."
  type        = string

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

variable "namespace" {
  description = "Kubernetes namespace KubeWatcher runs in."
  type        = string
  default     = "kubewatcher"
}

variable "chart_path" {
  description = "Filesystem path to the parent directory of all KubeWatcher Helm charts."
  type        = string
  default     = "../../../charts"
}

# ── Container images ──────────────────────────────────────────────
variable "image_registry" {
  description = "Registry prefix used for every component image."
  type        = string
  default     = "ghcr.io/drewjocham"
}

variable "image_tag" {
  description = "Image tag override; empty string lets the chart's appVersion win."
  type        = string
  default     = ""
}

# ── Releases ──────────────────────────────────────────────────────
variable "backend_enabled" {
  type    = bool
  default = true
}
variable "frontend_enabled" {
  type    = bool
  default = true
}
variable "agent_enabled" {
  type    = bool
  default = false
}
variable "alert_ingress_enabled" {
  type    = bool
  default = false
}
variable "mcp_enabled" {
  type    = bool
  default = false
}
variable "monitoring_enabled" {
  type    = bool
  default = false
}

# ── Backend configuration ─────────────────────────────────────────
variable "argocd_url" {
  description = "Argo CD server URL exposed to the backend."
  type        = string
  default     = ""
}

variable "anomstack_url" {
  description = "Anomstack service URL exposed to the backend."
  type        = string
  default     = ""
}

# ── Secrets ───────────────────────────────────────────────────────
variable "deepseek_api_key" {
  description = "DeepSeek API key for the backend AI features. Stored in a Kubernetes Secret."
  type        = string
  sensitive   = true
  default     = ""
}

variable "argocd_token" {
  description = "Argo CD API token for the backend. Stored in a Kubernetes Secret."
  type        = string
  sensitive   = true
  default     = ""
}

# ── Monitoring ────────────────────────────────────────────────────
variable "grafana_admin_password" {
  description = "Grafana admin password. Required when monitoring_enabled is true."
  type        = string
  sensitive   = true
  default     = ""
}

# ── Per-chart override hooks ──────────────────────────────────────
# Each accepts an arbitrary map merged with the module's computed
# values so callers can tweak without forking the module.
variable "extra_values_backend" {
  type    = any
  default = {}
}
variable "extra_values_frontend" {
  type    = any
  default = {}
}
variable "extra_values_agent" {
  type    = any
  default = {}
}
variable "extra_values_alert_ingress" {
  type    = any
  default = {}
}
variable "extra_values_mcp" {
  type    = any
  default = {}
}
variable "extra_values_monitoring" {
  type    = any
  default = {}
}
