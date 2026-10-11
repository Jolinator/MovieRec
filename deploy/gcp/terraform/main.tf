resource "google_project_service" "required" {
  for_each = toset([
    "compute.googleapis.com",
    "artifactregistry.googleapis.com",
    "iap.googleapis.com",
    "iam.googleapis.com",
    "cloudresourcemanager.googleapis.com",
  ])
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_compute_network" "movierec" {
  name                    = "${var.name}-network"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.required]
}

resource "google_compute_subnetwork" "movierec" {
  name          = "${var.name}-subnet"
  ip_cidr_range = "10.20.0.0/24"
  region        = var.region
  network       = google_compute_network.movierec.id
}

resource "google_compute_address" "movierec" {
  name       = "${var.name}-ip"
  region     = var.region
  depends_on = [google_project_service.required]
}

locals {
  # Use the shared local ports file, or the committed GCP example in a clean checkout.
  ports_env_contents = try(
    file("${path.module}/../../../secrets/ports.env"),
    file("${path.module}/../secrets-examples/ports.env.example"),
  )
  ports_env = {
    for line in split("\n", trimspace(local.ports_env_contents)) :
    trimspace(split("=", line)[0]) => tonumber(trimspace(split("=", line)[1]))
    if length(split("=", line)) == 2
  }
}

resource "google_service_account" "node" {
  account_id   = "${var.name}-node"
  display_name = "MovieRec Kubernetes VM"
  depends_on   = [google_project_service.required]
}

resource "google_compute_firewall" "web" {
  name    = "${var.name}-web"
  network = google_compute_network.movierec.name
  allow {
    protocol = "tcp"
    ports    = [tostring(local.ports_env["GCP_HTTPS_PORT"])]
  }
  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["${var.name}-node"]
}

resource "google_compute_firewall" "admin" {
  name    = "${var.name}-admin"
  network = google_compute_network.movierec.name
  allow {
    protocol = "tcp"
    ports    = [tostring(local.ports_env["GCP_SSH_PORT"])]
  }
  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["${var.name}-node"]
}

resource "google_artifact_registry_repository" "images" {
  location      = var.region
  repository_id = var.name
  description   = "MovieRec container images"
  format        = "DOCKER"
  depends_on    = [google_project_service.required]
}

# K3s on a self-managed VM does not automatically use Artifact Registry's
# credential helper. Public read access lets containerd pull images without
# storing a long-lived key on the node. Never put secrets in these images.
resource "google_artifact_registry_repository_iam_member" "public_reader" {
  for_each   = var.public_registry ? toset(["allUsers"]) : toset([])
  project    = var.project_id
  location   = google_artifact_registry_repository.images.location
  repository = google_artifact_registry_repository.images.repository_id
  role       = "roles/artifactregistry.reader"
  member     = each.value
}

resource "google_compute_instance" "node" {
  name         = "${var.name}-k8s"
  machine_type = var.machine_type
  zone         = var.zone
  tags         = ["${var.name}-node"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = 50
      type  = "pd-balanced"
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.movierec.id
    access_config {
      nat_ip = google_compute_address.movierec.address
    }
  }

  service_account {
    email  = google_service_account.node.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    k3s_tls_san = google_compute_address.movierec.address
  }
  metadata_startup_script = file("${path.module}/../../../scripts/k3s-startup.sh")

  depends_on = [google_project_service.required]
}
