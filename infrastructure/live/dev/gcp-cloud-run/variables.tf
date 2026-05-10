variable "project_id" { type = string }
variable "region"     { type = string; default = "us-west1" }

variable "service_account_email" {
  description = "Service account attached to every Cloud Run revision."
  type        = string
}

variable "backend_image"  { type = string }
variable "frontend_image" { type = string }
variable "mcp_image"      { type = string }

variable "flink_gateway_url" {
  description = "URL of the Flink gateway. Typically remote-state output from gcp-flink-vm."
  type        = string
}

variable "prometheus_url" {
  type    = string
  default = ""
}

variable "vpc_connector" {
  description = "VPC access connector for the backend's private network egress."
  type        = string
  default     = ""
}
