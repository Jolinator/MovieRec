output "public_ip" {
  value = google_compute_address.movierec.address
}

output "artifact_registry" {
  value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}"
}

output "app_instance_name" {
  value = google_compute_instance.app.name
}

output "database_instance_name" {
  value = google_compute_instance.database.name
}

output "model_instance_name" {
  value = google_compute_instance.model.name
}

output "app_private_ip" {
  value = google_compute_instance.app.network_interface[0].network_ip
}

output "region" {
  value = var.region
}

output "zone" {
  value = var.zone
}
