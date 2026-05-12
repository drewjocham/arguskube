# ── General ────────────────────────────────────────────────────────
variable "environment" {
  description = "Deployment environment (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "tags" {
  description = "Common resource tags"
  type        = map(string)
  default = {
    Project     = "kubewatcher"
    ManagedBy   = "terraform"
    Environment = "dev"
  }
}

# ── AWS Region ────────────────────────────────────────────────────
variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-west-2"
}

# ── VPC ───────────────────────────────────────────────────────────
variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "Availability zones for subnets"
  type        = list(string)
  default     = ["us-west-2a", "us-west-2b", "us-west-2c"]
}

variable "private_subnet_cidrs" {
  description = "Private subnet CIDRs"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
}

variable "public_subnet_cidrs" {
  description = "Public subnet CIDRs (optional — use private subnets with NAT)"
  type        = list(string)
  default     = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]
}

# ── EKS Cluster ──────────────────────────────────────────────────
variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
  default     = "kubewatcher"
}

variable "cluster_version" {
  description = "Kubernetes version"
  type        = string
  default     = "1.30"
}

variable "node_group_instance_types" {
  description = "Instance types for the node group"
  type        = list(string)
  default     = ["t3.medium"]
}

variable "node_group_desired_size" {
  description = "Desired node count"
  type        = number
  default     = 2
}

variable "node_group_min_size" {
  description = "Minimum node count (0 = scale to 0 at night)"
  type        = number
  default     = 0
}

variable "node_group_max_size" {
  description = "Maximum node count"
  type        = number
  default     = 6
}

# ── Vector Sinks ─────────────────────────────────────────────────
variable "vector_s3_alerts_bucket" {
  description = "S3 bucket for alert archival (Vector sink)"
  type        = string
  default     = ""
}

variable "vector_s3_logs_bucket" {
  description = "S3 bucket for agent log archival (Vector sidecar sink)"
  type        = string
  default     = ""
}

variable "vector_gcp_project" {
  description = "GCP project for PubSub alert publishing (Vector sink)"
  type        = string
  default     = ""
}

variable "vector_gcp_topic" {
  description = "GCP PubSub topic for alerts"
  type        = string
  default     = "argus-alerts"
}

variable "vector_agent_enabled" {
  description = "Enable Vector sidecar on the agent DaemonSet"
  type        = bool
  default     = false
}

# ── Helm Releases ─────────────────────────────────────────────────
variable "helm_backend_enabled" {
  description = "Enable backend Helm release"
  type        = bool
  default     = true
}

variable "helm_frontend_enabled" {
  description = "Enable frontend Helm release"
  type        = bool
  default     = true
}

variable "helm_agent_enabled" {
  description = "Enable agent Helm release (set to false if no agent needed)"
  type        = bool
  default     = false
}

variable "helm_alert_ingress_enabled" {
  description = "Enable alert-ingress Helm release (Vector pipeline)"
  type        = bool
  default     = false
}

variable "helm_mcp_enabled" {
  description = "Enable MCP server Helm release"
  type        = bool
  default     = false
}

variable "helm_monitoring_enabled" {
  description = "Enable monitoring stack Helm release"
  type        = bool
  default     = false
}

variable "helm_values_backend" {
  description = "Override values for backend chart"
  type        = any
  default     = {}
}

variable "helm_values_frontend" {
  description = "Override values for frontend chart"
  type        = any
  default     = {}
}

variable "helm_values_agent" {
  description = "Override values for agent chart"
  type        = any
  default     = {}
}

variable "helm_values_alert_ingress" {
  description = "Override values for alert-ingress chart"
  type        = any
  default     = {}
}

variable "helm_values_mcp" {
  description = "Override values for MCP server chart"
  type        = any
  default     = {}
}

variable "helm_values_monitoring" {
  description = "Override values for monitoring chart"
  type        = any
  default     = {}
}

# ── Container Registry ────────────────────────────────────────────
variable "image_registry" {
  description = "Container image registry prefix"
  type        = string
  default     = "ghcr.io/drewjocham"
}

variable "image_tag" {
  description = "Container image tag (defaults to chart appVersion)"
  type        = string
  default     = ""
}

# ── Secrets (dev only — use AWS Secrets Manager in prod) ──────────
variable "deepseek_api_key" {
  description = "DeepSeek API key for AI diagnostics"
  type        = string
  sensitive   = true
  default     = ""
}

variable "argocd_token" {
  description = "ArgoCD API token"
  type        = string
  sensitive   = true
  default     = ""
}

variable "argocd_url" {
  description = "ArgoCD server URL"
  type        = string
  default     = ""
}

variable "anomstack_url" {
  description = "Anomstack service URL"
  type        = string
  default     = ""
}
