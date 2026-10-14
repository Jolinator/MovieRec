variable "project_id" {
  description = "Google Cloud project ID (not the display name)."
  type        = string
}

variable "region" {
  type = string
}

variable "zone" {
  description = "All three VMs and their persistent disks use this zone."
  type        = string
}

variable "name" {
  type    = string
  default = "movierec"
  validation {
    condition     = can(regex("^[a-z]([-a-z0-9]*[a-z0-9])?$", var.name))
    error_message = "name must be a lowercase GCP resource name."
  }
}

variable "public_registry" {
  description = "Allow anonymous pulls from Artifact Registry (the app images become public)."
  type        = bool
  default     = false
}

variable "machine_type" {
  description = "Deprecated compatibility setting: use the role-specific machine type variables."
  type        = string
  default     = null
  nullable    = true
}

variable "app_machine_type" {
  description = "Machine type for the app/control-plane VM."
  type        = string
  default     = "e2-standard-2"
}

variable "database_machine_type" {
  description = "Machine type for the PostgreSQL VM."
  type        = string
  default     = "e2-standard-2"
}

variable "model_machine_type" {
  description = "Legacy/default CPU machine type for the model VM. Use model_cpu_machine_type for new configurations."
  type        = string
  default     = "e2-standard-2"
}

variable "model_cpu_machine_type" {
  description = "Machine type for the model VM when GPU support is disabled. This value is preserved when switching GPU types on and off."
  type        = string
  default     = null
  nullable    = true
}

variable "model_gpu_enabled" {
  description = "Provision GPU hardware for the model VM only. Driver/runtime setup and the GPU manifest are separate steps."
  type        = bool
  default     = false
}

variable "model_gpu_type" {
  description = "NVIDIA GPU model for the model VM. Select a model exposed in the configured zone."
  type        = string
  default     = "nvidia-l4"

  validation {
    condition = contains([
      "nvidia-b200",
      "nvidia-h100-80gb",
      "nvidia-h100-mega-80gb",
      "nvidia-h200-141gb",
      "nvidia-l4",
      "nvidia-l4-vws",
      "nvidia-rtx-pro-6000",
      "nvidia-rtx-pro-6000-vws",
      "nvidia-tesla-t4",
      "nvidia-tesla-t4-vws",
    ], var.model_gpu_type)
    error_message = "model_gpu_type must be an NVIDIA GPU type supported by this deployment; ct5lp and ct5p are TPUs, not GPUs."
  }
}

variable "model_gpu_machine_type" {
  description = "Optional machine type override for the selected model GPU. Defaults to a recommended compatible type."
  type        = string
  default     = null
  nullable    = true
}

variable "model_gpu_count" {
  description = "GPU count for N1-attached GPUs such as T4. Accelerator-optimized machine types get their GPU count from the machine type."
  type        = number
  default     = 1

  validation {
    condition     = var.model_gpu_count >= 1 && var.model_gpu_count <= 4 && floor(var.model_gpu_count) == var.model_gpu_count
    error_message = "model_gpu_count must be a whole number from 1 to 4."
  }
}

variable "app_data_disk_gb" {
  type    = number
  default = 20
}

variable "database_data_disk_gb" {
  type    = number
  default = 30
}

variable "model_data_disk_gb" {
  type    = number
  default = 20
}
