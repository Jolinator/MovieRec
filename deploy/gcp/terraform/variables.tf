variable "project_id" {
  description = "Google Cloud project ID (not the display name)."
  type        = string
}

variable "region" {
  type    = string
  default = "europe-west1"
}

variable "zone" {
  type    = string
  default = "europe-west1-b"
}

variable "name" {
  type    = string
  default = "movierec"
}

variable "public_registry" {
  description = "Allow anonymous pulls from Artifact Registry (the app images become public)."
  type        = bool
  default     = false
}

variable "machine_type" {
  description = "Use a small machine for a demo; increase if pods are evicted for memory pressure."
  type        = string
  default     = "e2-standard-2"
}
