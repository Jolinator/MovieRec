#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
BACKEND_FILE="${TF_DIR}/backend.tf"
STATE_PREFIX="g21/terraform"
MIN_STATE_RESOURCES=20
REQUIRED_STATE_RESOURCES=(
  "google_artifact_registry_repository.images"
  "google_compute_address.movierec"
  "google_compute_disk.app_data"
  "google_compute_disk.database_data"
  "google_compute_disk.model_data"
  "google_compute_instance.app"
  "google_compute_instance.database"
  "google_compute_network.movierec"
  "google_service_account.node"
)

usage() {
  cat <<'EOF'
Configure or join the shared GCS Terraform state for this project.

Usage:
  bash scripts/gcp/terraform-backend.sh setup [--project ID] [--bucket NAME]
       [--location LOCATION] [--admin EMAIL ...]
  bash scripts/gcp/terraform-backend.sh join

Run `setup` once from the shell with the complete Terraform state. It creates
the bucket, grants all listed admins access, writes backend.tf, and migrates
that state. Commit and share backend.tf; the other admins then run `join`.

Options for setup:
  --project ID         Defaults to the existing GCP project resolver.
  --bucket NAME        Defaults to <project>-g21-tfstate.
  --location LOCATION  GCS location; defaults to EU.
  --admin EMAIL        Add an admin account; repeat as needed. Without this
                       option, prompts for emails until you type 'stop'.
EOF
}

fail() {
  printf 'terraform-backend: %s\n' "$*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Required command not found: $1"
}

parse_setup_args() {
  SETUP_PROJECT="${GCP_PROJECT_ID:-}"
  SETUP_BUCKET=""
  SETUP_LOCATION="EU"
  PROVIDED_ADMINS=()

  while (($#)); do
    case "$1" in
      --project)
        (($# >= 2)) || fail "Missing value for --project"
        SETUP_PROJECT="$2"
        shift 2
        ;;
      --bucket)
        (($# >= 2)) || fail "Missing value for --bucket"
        SETUP_BUCKET="$2"
        shift 2
        ;;
      --location)
        (($# >= 2)) || fail "Missing value for --location"
        SETUP_LOCATION="$2"
        shift 2
        ;;
      --admin)
        (($# >= 2)) || fail "Missing value for --admin"
        PROVIDED_ADMINS+=("$2")
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "Unknown setup option: $1 (use --help for usage)"
        ;;
    esac
  done
}

project_id_for_setup() {
  SETUP_PROJECT="$(GCP_PROJECT_ID="${SETUP_PROJECT}" bash "${SCRIPT_DIR}/gcp-context.sh" project-id)" \
    || fail "Could not resolve the GCP project. Set GCP_PROJECT_ID or select it with 'gcloud config set project PROJECT_ID'."
}

active_account() {
  local account
  account="$(gcloud auth list --filter='status:ACTIVE' --format='value(account)' | sed -n '1p')"
  [[ -n "${account}" ]] || fail "No active gcloud account. Authenticate with 'gcloud auth login'."
  printf '%s' "${account}"
}

validate_email() {
  [[ "$1" =~ ^[^[:space:]@]+@[^[:space:]@]+$ ]] || fail "Invalid Google account email: $1"
}

add_admin_email() {
  local email="$1"
  local existing
  validate_email "${email}"
  for existing in "${ADMIN_EMAILS[@]}"; do
    if [[ "${existing,,}" == "${email,,}" ]]; then
      printf 'Already included: %s\n' "${email}"
      return
    fi
  done
  ADMIN_EMAILS+=("${email}")
}

collect_admin_emails() {
  local active_account="$1"
  local email
  ADMIN_EMAILS=()
  add_admin_email "${active_account}"

  if ((${#PROVIDED_ADMINS[@]})); then
    for email in "${PROVIDED_ADMINS[@]}"; do
      add_admin_email "${email}"
    done
  elif [[ -t 0 ]]; then
    printf 'Included active account: %s\n' "${active_account}"
    while true; do
      read -r -p "Additional admin email (or type 'stop' to finish): " email
      email="${email#"${email%%[![:space:]]*}"}"
      email="${email%"${email##*[![:space:]]}"}"
      case "${email,,}" in
        stop|done)
          break
          ;;
        "")
          printf "Enter an email or type 'stop'.\n"
          ;;
        *)
          if [[ "${email}" =~ ^[^[:space:]@]+@[^[:space:]@]+$ ]]; then
            add_admin_email "${email}"
          else
            printf "%s\n" "That does not look like an email address. Try again or type 'stop'."
          fi
          ;;
      esac
    done
  else
    fail "Pass --admin EMAIL once for each additional admin when running non-interactively."
  fi

  if ((${#ADMIN_EMAILS[@]} < 2)); then
    fail "Add at least one additional admin before stopping."
  fi
}

cached_backend_type() {
  local cache_file="${TF_DIR}/.terraform/terraform.tfstate"
  if [[ ! -f "${cache_file}" ]]; then
    printf 'local'
    return
  fi
  python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8")).get("backend", {}).get("type", "unknown"))' "${cache_file}"
}

check_default_workspace() {
  local current
  current="$(terraform -chdir="${TF_DIR}" workspace show 2>/dev/null)" \
    || fail "Could not read the Terraform workspace. Run terraform init and try again."
  [[ "${current}" == "default" ]] \
    || fail "This helper expects the default workspace; current workspace is '${current}'."

  local workspaces
  workspaces="$(terraform -chdir="${TF_DIR}" workspace list | sed -E 's/^[*[:space:]]+//' | sed '/^$/d')"
  [[ "${workspaces}" == "default" ]] \
    || fail "This helper only migrates the default workspace. Found: ${workspaces//$'\n'/, }"
}

read_state_addresses() {
  STATE_ADDRESSES_TEXT="$(terraform -chdir="${TF_DIR}" state list 2>&1)" \
    || fail "Cannot read the current Terraform state:\n${STATE_ADDRESSES_TEXT}"
  STATE_ADDRESSES=()
  if [[ -n "${STATE_ADDRESSES_TEXT}" ]]; then
    mapfile -t STATE_ADDRESSES <<<"${STATE_ADDRESSES_TEXT}"
  fi
}

validate_full_state() {
  read_state_addresses
  local count="${#STATE_ADDRESSES[@]}"
  ((count >= MIN_STATE_RESOURCES)) \
    || fail "This state has ${count} resources; expected at least ${MIN_STATE_RESOURCES}. Use setup only from the shell with the complete infrastructure state."

  local required found address
  for required in "${REQUIRED_STATE_RESOURCES[@]}"; do
    found=0
    for address in "${STATE_ADDRESSES[@]}"; do
      if [[ "${address}" == "${required}" ]]; then
        found=1
        break
      fi
    done
    ((found == 1)) || fail "State is missing ${required}; refusing to migrate a partial state."
  done
}

validate_shared_state() {
  validate_full_state
  printf 'Shared Terraform state is active (%s resources).\n' "${#STATE_ADDRESSES[@]}"
}

state_backup_path() {
  printf '%s/terraform.pre-gcs-%s.tfstate' "${TF_DIR}" "$(date -u +%Y%m%dT%H%M%SZ)"
}

backup_active_state() {
  local backup
  backup="$(state_backup_path)"
  (umask 077; terraform -chdir="${TF_DIR}" state pull >"${backup}") \
    || fail "Could not create a local state backup. No backend migration was started."
  chmod 600 "${backup}"
  printf 'Private state backup: %s\n' "${backup}"
}

ensure_bucket() {
  local bucket_url="$1"
  local project_number bucket_project_number

  project_number="$(gcloud projects describe "${SETUP_PROJECT}" --format='value(projectNumber)')" \
    || fail "Cannot read project ${SETUP_PROJECT}. Check the active account and project access."
  bucket_project_number="$(gcloud storage buckets describe "${bucket_url}" --format='value(projectNumber)' 2>/dev/null || true)"

  if [[ -n "${bucket_project_number}" ]]; then
    [[ "${bucket_project_number}" == "${project_number}" ]] \
      || fail "Bucket ${bucket_url} belongs to a different project. Choose another bucket name."
    printf 'Using existing bucket %s.\n' "${bucket_url}"
  else
    printf 'Creating bucket %s in %s.\n' "${bucket_url}" "${SETUP_LOCATION}"
    gcloud storage buckets create "${bucket_url}" \
      --project="${SETUP_PROJECT}" \
      --location="${SETUP_LOCATION}" \
      --uniform-bucket-level-access
  fi

  gcloud storage buckets update "${bucket_url}" --versioning
  local admin_email
  for admin_email in "${ADMIN_EMAILS[@]}"; do
    gcloud storage buckets add-iam-policy-binding "${bucket_url}" \
      --member="user:${admin_email}" --role="roles/storage.objectAdmin"
  done
}

write_backend_config() {
  local bucket="$1"
  local temporary_file="${BACKEND_FILE}.tmp.$$"
  cat >"${temporary_file}" <<EOF
terraform {
  backend "gcs" {
    bucket = "${bucket}"
    prefix = "${STATE_PREFIX}"
  }
}
EOF
  mv -- "${temporary_file}" "${BACKEND_FILE}"
}

setup_backend() {
  parse_setup_args "$@"
  need_command gcloud
  need_command terraform
  need_command python3
  need_command sed

  [[ ! -e "${BACKEND_FILE}" ]] \
    || fail "${BACKEND_FILE} already exists. Run 'join' from the other admin's shell, or inspect the existing backend setup."

  project_id_for_setup
  CURRENT_ADMIN="$(active_account)"
  collect_admin_emails "${CURRENT_ADMIN}"

  if [[ -z "${SETUP_BUCKET}" ]]; then
    local suggested_bucket="${SETUP_PROJECT}-g21-tfstate"
    if [[ -t 0 ]]; then
      read -r -p "GCS bucket name [${suggested_bucket}]: " SETUP_BUCKET
      SETUP_BUCKET="${SETUP_BUCKET:-${suggested_bucket}}"
    else
      SETUP_BUCKET="${suggested_bucket}"
    fi
  fi
  [[ "${SETUP_BUCKET}" =~ ^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$ ]] \
    || fail "Bucket name must be 3-63 lowercase letters, numbers, or hyphens, with alphanumeric first and last characters."

  local cached_type
  cached_type="$(cached_backend_type)"
  [[ "${cached_type}" == "local" ]] \
    || fail "Terraform is currently initialized with the '${cached_type}' backend. Refusing to migrate from an unexpected backend."

  terraform -chdir="${TF_DIR}" init
  check_default_workspace
  validate_full_state

  printf '\nProject:       %s\n' "${SETUP_PROJECT}"
  printf 'Bucket:        gs://%s (%s)\n' "${SETUP_BUCKET}" "${SETUP_LOCATION}"
  printf 'State source:  local default workspace (%s resources)\n' "${#STATE_ADDRESSES[@]}"
  printf 'Admins:\n'
  printf '  %s\n' "${ADMIN_EMAILS[@]}"
  printf '\n'
  if [[ -t 0 ]]; then
    local confirmation
    read -r -p "Type MIGRATE to create/configure the bucket and move this state: " confirmation
    [[ "${confirmation}" == "MIGRATE" ]] || fail "Setup cancelled."
  else
    fail "Setup requires an interactive confirmation. Run it in Cloud Shell or a terminal."
  fi

  ensure_bucket "gs://${SETUP_BUCKET}"
  backup_active_state
  write_backend_config "${SETUP_BUCKET}"

  printf '\nMigrating Terraform state to gs://%s/%s.\n' "${SETUP_BUCKET}" "${STATE_PREFIX}"
  terraform -chdir="${TF_DIR}" init -migrate-state
  validate_shared_state

  cat <<EOF

Shared state is ready. Commit and share:
  ${BACKEND_FILE}

After the other admin pulls that file, they should run:
  make tf-backend-join
EOF
}

read_backend_bucket() {
  [[ -f "${BACKEND_FILE}" ]] \
    || fail "${BACKEND_FILE} is missing. The state-owning admin must run 'make tf-backend-setup' and share the generated file first."
  local buckets
  buckets="$(sed -nE 's/^[[:space:]]*bucket[[:space:]]*=[[:space:]]*"([a-z0-9-]+)"[[:space:]]*$/\1/p' "${BACKEND_FILE}")"
  [[ -n "${buckets}" && "$(printf '%s\n' "${buckets}" | wc -l | tr -d ' ')" == 1 ]] \
    || fail "Could not read exactly one GCS bucket from ${BACKEND_FILE}."
  BACKEND_BUCKET="${buckets}"
}

backup_local_files() {
  local timestamp source backup found=0
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  local state_files=("${TF_DIR}/terraform.tfstate" "${TF_DIR}/terraform.tfstate.d"/*/terraform.tfstate)
  for source in "${state_files[@]}"; do
    [[ -f "${source}" ]] || continue
    backup="${source%.tfstate}.pre-shared-${timestamp}.tfstate"
    (umask 077; cp -- "${source}" "${backup}")
    chmod 600 "${backup}"
    printf 'Saved local state copy: %s\n' "${backup}"
    found=1
  done
  if ((found == 0)); then
    printf 'No local state file found; joining will use the shared state only.\n'
  fi
}

join_backend() {
  (($# == 0)) || fail "The join command takes no options."
  need_command gcloud
  need_command terraform
  need_command sed

  read_backend_bucket
  gcloud storage buckets describe "gs://${BACKEND_BUCKET}" >/dev/null \
    || fail "Cannot access gs://${BACKEND_BUCKET}. Check the backend file and your Google Cloud account."

  backup_local_files
  printf 'Connecting this shell to gs://%s/%s without migrating its local state.\n' "${BACKEND_BUCKET}" "${STATE_PREFIX}"
  terraform -chdir="${TF_DIR}" init -reconfigure
  validate_shared_state
  printf 'This shell is now using the shared Terraform state.\n'
}

main() {
  local command="${1:-}"
  (($# > 0)) && shift
  case "${command}" in
    setup)
      setup_backend "$@"
      ;;
    join)
      join_backend "$@"
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      usage >&2
      fail "Choose 'setup' or 'join'."
      ;;
  esac
}

main "$@"
