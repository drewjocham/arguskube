variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-west1"
}

variable "zone" {
  description = "GCP zone"
  type        = string
  default     = "us-west1-a"
}

variable "name_prefix" {
  description = "Name prefix for all resources"
  type        = string
  default     = "kubewatcher"
}

variable "machine_type" {
  description = "Compute Engine machine type for Flink"
  type        = string
  default     = "e2-standard-4"
}

variable "disk_size_gb" {
  description = "Boot disk size in GB"
  type        = number
  default     = 100
}

variable "network" {
  description = "VPC network name"
  type        = string
}

variable "subnetwork" {
  description = "VPC subnetwork name"
  type        = string
}

variable "internal_cidr" {
  description = "Internal CIDR range for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "assign_public_ip" {
  description = "Assign a public IP to the Flink VM"
  type        = bool
  default     = false
}

variable "allow_gateway_ingress" {
  description = "Allow external ingress to the gateway port"
  type        = bool
  default     = false
}

variable "gateway_allowed_cidrs" {
  description = "CIDR blocks allowed to access the Flink gateway"
  type        = list(string)
  default     = []
}

variable "flink_version" {
  description = "Apache Flink version to deploy"
  type        = string
  default     = "1.18.0"
}

variable "gateway_port" {
  description = "Flink gateway service port"
  type        = string
  default     = "8087"
}

variable "gateway_api_key" {
  description = "API key for Flink gateway authentication"
  type        = string
  sensitive  = true
}

variable "service_account_email" {
  description = "Service account email for the Flink VM"
  type        = string
  default     = null
}

variable "kafka_bootstrap_servers" {
  description = "Kafka bootstrap servers for metric ingestion"
  type        = string
  default     = "localhost:9092"
}

variable "monitoring_enabled" {
  description = "Enable Cloud Monitoring integration"
  type        = bool
  default     = true
}
