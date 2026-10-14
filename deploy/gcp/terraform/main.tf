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
  # Use deployment-specific ports, or the committed example in a clean checkout.
  ports_env_contents = try(
    file("${path.module}/../../ports.env"),
    file("${path.module}/../../ports.env.example"),
  )
  ports_env = {
    for line in split("\n", trimspace(local.ports_env_contents)) :
    trimspace(split("=", line)[0]) => tonumber(trimspace(split("=", line)[1]))
    if length(split("=", line)) == 2
  }
  app_machine_type      = var.machine_type != null ? var.machine_type : var.app_machine_type
  database_machine_type = var.machine_type != null ? var.machine_type : var.database_machine_type
  model_gpu_base_type   = trimsuffix(var.model_gpu_type, "-vws")
  model_gpu_machine_type_defaults = {
    "nvidia-b200"           = "a4-highgpu-8g"
    "nvidia-h100-80gb"      = "a3-highgpu-1g"
    "nvidia-h100-mega-80gb" = "a3-megagpu-8g"
    "nvidia-h200-141gb"     = "a3-ultragpu-8g"
    "nvidia-l4"             = "g2-standard-4"
    "nvidia-rtx-pro-6000"   = "g4-standard-48"
    "nvidia-tesla-t4"       = "n1-standard-4"
  }
  model_gpu_machine_type_prefixes = {
    "nvidia-b200"           = "a4-highgpu-"
    "nvidia-h100-80gb"      = "a3-highgpu-"
    "nvidia-h100-mega-80gb" = "a3-megagpu-"
    "nvidia-h200-141gb"     = "a3-ultragpu-"
    "nvidia-l4"             = "g2-"
    "nvidia-rtx-pro-6000"   = "g4-"
    "nvidia-tesla-t4"       = "n1-"
  }
  model_gpu_machine_type = (
    var.model_gpu_machine_type != null && var.model_gpu_machine_type != ""
    ? var.model_gpu_machine_type
    : local.model_gpu_machine_type_defaults[local.model_gpu_base_type]
  )
  # The former gpu-enable script replaced model_machine_type with a G2 type.
  # If no separate CPU type was recorded, restore the example's original default.
  model_cpu_machine_type = var.model_cpu_machine_type != null ? var.model_cpu_machine_type : (
    can(regex("^g2-", var.model_machine_type)) ? "e2-standard-2" : var.model_machine_type
  )
  model_machine_type = var.model_gpu_enabled ? local.model_gpu_machine_type : (
    var.machine_type != null ? var.machine_type : local.model_cpu_machine_type
  )
  model_gpu_uses_guest_accelerator = local.model_gpu_base_type == "nvidia-tesla-t4" || (
    endswith(var.model_gpu_type, "-vws") && contains(
      ["nvidia-l4", "nvidia-rtx-pro-6000"],
      local.model_gpu_base_type,
    )
  )
}

resource "google_compute_router" "movierec" {
  name    = "${var.name}-router"
  region  = var.region
  network = google_compute_network.movierec.id
}

# Worker VMs have no public addresses. NAT lets them install packages and pull
# upstream images/models while keeping their interfaces private.
resource "google_compute_router_nat" "movierec" {
  name                               = "${var.name}-nat"
  router                             = google_compute_router.movierec.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"
  subnetwork {
    name                    = google_compute_subnetwork.movierec.id
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
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
  target_tags   = ["${var.name}-app"]
}

resource "google_compute_firewall" "admin" {
  name    = "${var.name}-admin"
  network = google_compute_network.movierec.name
  allow {
    protocol = "tcp"
    ports    = [tostring(local.ports_env["GCP_SSH_PORT"])]
  }
  # IAP TCP forwarding source range. SSH is not exposed to the public internet.
  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["${var.name}-node"]
}

resource "google_compute_firewall" "k3s_api" {
  name    = "${var.name}-k3s-api"
  network = google_compute_network.movierec.name
  allow {
    protocol = "tcp"
    ports    = ["6443"]
  }
  source_ranges = ["10.20.0.0/24"]
  target_tags   = ["${var.name}-app"]
}

resource "google_compute_firewall" "k3s_flannel" {
  name    = "${var.name}-k3s-flannel"
  network = google_compute_network.movierec.name
  allow {
    protocol = "udp"
    ports    = ["8472"]
  }
  source_ranges = ["10.20.0.0/24"]
  target_tags   = ["${var.name}-node"]
}

resource "google_compute_firewall" "k3s_kubelet" {
  name    = "${var.name}-k3s-kubelet"
  network = google_compute_network.movierec.name
  allow {
    protocol = "tcp"
    ports    = ["10250"]
  }
  source_ranges = ["10.20.0.0/24"]
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
# storing a long-lived key on the nodes. Never put secrets in these images.
resource "google_artifact_registry_repository_iam_member" "public_reader" {
  for_each   = var.public_registry ? toset(["allUsers"]) : toset([])
  project    = var.project_id
  location   = google_artifact_registry_repository.images.location
  repository = google_artifact_registry_repository.images.repository_id
  role       = "roles/artifactregistry.reader"
  member     = each.value
}

resource "google_compute_disk" "app_data" {
  name = "${var.name}-app-data"
  type = "pd-balanced"
  zone = var.zone
  size = var.app_data_disk_gb
}

resource "google_compute_disk" "database_data" {
  name = "${var.name}-database-data"
  type = "pd-balanced"
  zone = var.zone
  size = var.database_data_disk_gb
}

resource "google_compute_disk" "model_data" {
  name = "${var.name}-model-data"
  type = "pd-balanced"
  zone = var.zone
  size = var.model_data_disk_gb
}

resource "google_compute_instance" "app" {
  name                      = "${var.name}-app"
  machine_type              = local.app_machine_type
  zone                      = var.zone
  allow_stopping_for_update = true
  tags                      = ["${var.name}-node", "${var.name}-app"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = 50
      type  = "pd-balanced"
    }
  }

  attached_disk {
    source      = google_compute_disk.app_data.id
    device_name = "${var.name}-app-data"
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
    movierec_role    = "app"
    node_name        = "${var.name}-app"
    node_label       = "movierec.io/role=app"
    data_disk_device = "/dev/disk/by-id/google-${var.name}-app-data"
    gpu_enabled      = "false"
    k3s_tls_san      = google_compute_address.movierec.address
  }
  metadata_startup_script = file("${path.module}/../../../scripts/gcp/k3s-startup.sh")

  depends_on = [
    google_compute_router_nat.movierec,
    google_project_service.required,
  ]
}

resource "google_compute_instance" "database" {
  name                      = "${var.name}-database"
  machine_type              = local.database_machine_type
  zone                      = var.zone
  allow_stopping_for_update = true
  tags                      = ["${var.name}-node", "${var.name}-database"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = 30
      type  = "pd-balanced"
    }
  }

  attached_disk {
    source      = google_compute_disk.database_data.id
    device_name = "${var.name}-database-data"
  }

  network_interface {
    subnetwork = google_compute_subnetwork.movierec.id
  }

  service_account {
    email  = google_service_account.node.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    movierec_role    = "database"
    node_name        = "${var.name}-database"
    node_label       = "movierec.io/role=database"
    data_disk_device = "/dev/disk/by-id/google-${var.name}-database-data"
    gpu_enabled      = "false"
  }
  metadata_startup_script = file("${path.module}/../../../scripts/gcp/k3s-startup.sh")

  depends_on = [
    google_compute_router_nat.movierec,
    google_project_service.required,
  ]
}

resource "google_compute_instance" "model" {
  name                      = "${var.name}-model"
  machine_type              = local.model_machine_type
  zone                      = var.zone
  allow_stopping_for_update = true
  tags                      = ["${var.name}-node", "${var.name}-model"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = var.model_gpu_enabled ? 50 : 30
      type  = "pd-balanced"
    }
  }

  attached_disk {
    source      = google_compute_disk.model_data.id
    device_name = "${var.name}-model-data"
  }

  scheduling {
    on_host_maintenance = var.model_gpu_enabled ? "TERMINATE" : "MIGRATE"
  }

  dynamic "guest_accelerator" {
    for_each = var.model_gpu_enabled && local.model_gpu_uses_guest_accelerator ? [1] : []
    content {
      type  = "projects/${var.project_id}/zones/${var.zone}/acceleratorTypes/${var.model_gpu_type}"
      count = var.model_gpu_count
    }
  }

  lifecycle {
    precondition {
      condition = !var.model_gpu_enabled || can(regex(
        "^${local.model_gpu_machine_type_prefixes[local.model_gpu_base_type]}",
        local.model_machine_type,
      ))
      error_message = "The selected model GPU type requires its matching GPU machine family. Set GPU_MACHINE_TYPE to a compatible type."
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.movierec.id
  }

  service_account {
    email  = google_service_account.node.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    movierec_role    = "model"
    node_name        = "${var.name}-model"
    node_label       = "movierec.io/role=model"
    data_disk_device = "/dev/disk/by-id/google-${var.name}-model-data"
    gpu_enabled      = tostring(var.model_gpu_enabled)
  }
  metadata_startup_script = file("${path.module}/../../../scripts/gcp/k3s-startup.sh")

  depends_on = [
    google_compute_router_nat.movierec,
    google_project_service.required,
  ]
}
