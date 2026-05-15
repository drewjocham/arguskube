variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "run_id" {
  description = "Unique run identifier. Used for resource naming and labels."
  type        = string
}

variable "region" {
  description = "GCP region for the cluster."
  type        = string
}

variable "node_count" {
  description = "Number of spot nodes in the pool."
  type        = number
  default     = 1
}

variable "machine_type" {
  description = "GCE machine type for spot nodes."
  type        = string
  default     = "e2-small"
}

variable "broker_kind" {
  description = "Kind of broker to install via Helm (nats|kafka|rabbitmq|pubsub|amqp1|rest)."
  type        = string
  default     = "nats"
}

variable "broker_values" {
  description = "Additional Helm values YAML for the broker chart."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Labels applied to every resource."
  type        = map(string)
  default     = {}
}
