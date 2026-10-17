variable "project_id" {
  description = "Google Cloud project ID (not the display name)."
  type        = string
}

variable "region" {
  type = string
}

variable "artifact_registry_region" {
  description = "Artifact Registry location. Defaults to region, but can stay fixed while compute moves."
  type        = string
  nullable    = true
}

variable "zone" {
  description = "Zone for the app/database VMs and their persistent disks."
  type        = string
}

variable "model_zone" {
  description = "Zone for the model VM and its data disk. Defaults to zone when unset."
  type        = string
  nullable    = true
}

variable "app_private_ip" {
  description = "Optional fixed private IP for the app VM, used to preserve the K3s API endpoint during a region move."
  type        = string
  nullable    = true
}

variable "app_boot_disk_snapshot" {
  description = "Optional snapshot used when recreating the app boot disk during a migration."
  type        = string
  nullable    = true
}

variable "database_boot_disk_snapshot" {
  description = "Optional snapshot used when recreating the database boot disk during a migration."
  type        = string
  nullable    = true
}

variable "model_boot_disk_snapshot" {
  description = "Optional snapshot used when recreating the model boot disk during a full-cluster migration."
  type        = string
  nullable    = true
}

variable "app_data_disk_snapshot" {
  description = "Optional snapshot used when recreating the app data disk during a migration."
  type        = string
  nullable    = true
}

variable "database_data_disk_snapshot" {
  description = "Optional snapshot used when recreating the database data disk during a migration."
  type        = string
  nullable    = true
}

variable "model_data_disk_snapshot" {
  description = "Optional snapshot used when recreating the model data disk during a full-cluster migration."
  type        = string
  nullable    = true
}

variable "name" {
  type    = string
  validation {
    condition     = can(regex("^[a-z]([-a-z0-9]*[a-z0-9])?$", var.name))
    error_message = "name must be a lowercase GCP resource name."
  }
}

variable "public_registry" {
  description = "Allow anonymous pulls from Artifact Registry (the app images become public)."
  type        = bool
}

variable "artifact_registry_keep_count" {
  description = "Number of recent versions to retain for each Artifact Registry image package."
  type        = number
  validation {
    condition     = var.artifact_registry_keep_count >= 1 && floor(var.artifact_registry_keep_count) == var.artifact_registry_keep_count
    error_message = "artifact_registry_keep_count must be a positive integer."
  }
}

variable "machine_type" {
  description = "Deprecated compatibility setting: use the role-specific machine type variables."
  type        = string
  nullable    = true
}

variable "app_machine_type" {
  description = "Machine type for the app/control-plane VM."
  type        = string
}

variable "database_machine_type" {
  description = "Machine type for the PostgreSQL VM."
  type        = string
}

variable "model_machine_type" {
  description = "Legacy/default CPU machine type for the model VM. Use model_cpu_machine_type for new configurations."
  type        = string
  nullable    = true
}

variable "model_cpu_machine_type" {
  description = "Machine type for the model VM when GPU support is disabled. This value is preserved when switching GPU types on and off."
  type        = string
  nullable    = true
}

variable "model_gpu_enabled" {
  description = "Provision GPU hardware for the model VM only. Driver/runtime setup and the GPU manifest are separate steps."
  type        = bool
}

variable "model_gpu_type" {
  description = "NVIDIA GPU model for the model VM. Select a model exposed in the configured zone."
  type        = string

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
  nullable    = true
}

variable "model_gpu_count" {
  description = "GPU count for N1-attached GPUs such as T4. Accelerator-optimized machine types get their GPU count from the machine type."
  type        = number
  validation {
    condition     = contains([1, 2, 4], var.model_gpu_count)
    error_message = "model_gpu_count must be 1, 2, or 4."
  }
}

variable "app_data_disk_gb" {
  type    = number
}

variable "database_data_disk_gb" {
  type    = number
}

variable "model_data_disk_gb" {
  type    = number
}
