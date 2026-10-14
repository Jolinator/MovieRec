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
  description = "Machine type for the model VM. Use a GPU-compatible type when model_gpu_enabled is true."
  type        = string
  default     = "e2-standard-2"
}

variable "model_gpu_enabled" {
  description = "Use a G2 model VM with its built-in NVIDIA L4 GPU. Driver/runtime setup and the GPU manifest are separate opt-in steps."
  type        = bool
  default     = false
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
