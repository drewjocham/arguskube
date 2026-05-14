variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "Region for the static IP and subnet."
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "Zone for the VM. Pick one with the GPU SKU you want."
  type        = string
  default     = "us-central1-a"
}

variable "name" {
  description = "Resource name prefix."
  type        = string
  default     = "argus-llm"
}

variable "machine_type" {
  description = "Compute Engine machine type. g2-standard-* gives you an L4 by default."
  type        = string
  default     = "g2-standard-8" # 8 vCPU, 32 GB RAM, 1 L4 (24 GB)
}

variable "gpu_type" {
  description = "GPU SKU. Common: nvidia-l4, nvidia-tesla-t4, nvidia-a100-80gb, nvidia-h100-80gb."
  type        = string
  default     = "nvidia-l4"
}

variable "gpu_count" {
  description = "How many GPUs to attach."
  type        = number
  default     = 1
}

variable "boot_image" {
  description = "Boot image. The DLVM family has CUDA + cuDNN preinstalled."
  type        = string
  default     = "deeplearning-platform-release/common-cu124-debian-11"
}

variable "disk_gb" {
  description = "Boot disk size — model weights + HF cache live here."
  type        = number
  default     = 200
}

variable "network" {
  description = "VPC network for the instance."
  type        = string
  default     = "default"
}

variable "subnetwork" {
  description = "Subnetwork. Leave null to use the default subnet for the region."
  type        = string
  default     = null
}

variable "allowed_source_ranges" {
  description = "CIDR blocks allowed to reach the LLM port + SSH. Default = your IP only via 0.0.0.0/32 placeholder; override!"
  type        = list(string)
  default     = ["0.0.0.0/32"]
}

variable "preemptible" {
  description = "Use spot pricing. ~70% cheaper, can be reclaimed by GCP."
  type        = bool
  default     = true
}

variable "model_name" {
  description = "HuggingFace model id served by vLLM."
  type        = string
  default     = "meta-llama/Llama-3.1-8B-Instruct"
}

variable "llm_api_key" {
  description = "Bearer token clients must present. Generate with: openssl rand -hex 32"
  type        = string
  sensitive   = true
}

variable "port" {
  description = "Port the LLM server listens on (vLLM default is 8000)."
  type        = number
  default     = 8000
}

variable "gpu_mem_fraction" {
  description = "Fraction of GPU memory vLLM may allocate."
  type        = number
  default     = 0.92
}

variable "hf_token" {
  description = "HuggingFace token, required only for gated/private models."
  type        = string
  default     = ""
  sensitive   = true
}

variable "service_account_email" {
  description = "Service account for the VM. Leave empty for the default Compute SA."
  type        = string
  default     = null
}

variable "service_account_scopes" {
  description = "OAuth scopes granted to the VM. Cloud Logging is enough for vLLM."
  type        = list(string)
  default     = ["https://www.googleapis.com/auth/logging.write"]
}
