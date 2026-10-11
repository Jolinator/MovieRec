output "public_ip" {
  value = google_compute_address.movierec.address
}

output "artifact_registry" {
  value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}"
}

output "instance_name" {
  value = google_compute_instance.node.name
}

output "region" {
  value = var.region
}

output "zone" {
  value = var.zone
}
