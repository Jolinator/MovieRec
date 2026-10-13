terraform {
  required_version = ">= 1.5.0"

  # Terraform provisions infrastructure. The CI deployment job discovers the
  # provisioned GCP resources directly and does not initialize Terraform.

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
