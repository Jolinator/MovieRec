terraform {
  required_version = ">= 1.5.0"

  # Shared by local Terraform and GitLab CI. Configure credentials and the
  # state address with TF_HTTP_* environment variables.
  backend "http" {}

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
