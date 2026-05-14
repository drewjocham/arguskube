variable "project_id" {
  description = "GCP project hosting the Flink VM."
  type        = string
}

variable "region" {
  description = "GCP region."
  type        = string
  default     = "us-west1"
}

variable "zone" {
  description = "GCP zone for the VM."
  type        = string
  default     = "us-west1-a"
}

variable "network" {
  description = "VPC network name."
  type        = string
}

variable "subnetwork" {
  description = "VPC subnetwork name."
  type        = string
}

variable "gateway_api_key" {
  description = "API key for the Flink anomaly-detection gateway."
  type        = string
  sensitive   = true
}

variable "kafka_bootstrap_servers" {
  description = "Kafka bootstrap servers Flink reads from."
  type        = string
  default     = "localhost:9092"
}
