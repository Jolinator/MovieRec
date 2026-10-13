terraform {
  required_version = ">= 1.5.0"

  # Manual Cloud Shell runs use Terraform's local state backend. GitLab CI
  # adds its HTTP backend explicitly before initialization.

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}
