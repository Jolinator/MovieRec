#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"

usage() {
  printf 'Usage: %s {app|database|model} [--command COMMAND] [--ssh-flag FLAG ...]\n' "${0##*/}"
  printf 'Without --command, opens an interactive SSH session.\n'
}

if (($# == 0)); then
  usage >&2
  exit 2
fi

machine="$1"
shift
remote_command=""
command_set=0
ssh_flags=()
while (($#)); do
  case "$1" in
    --command)
      (($# >= 2)) || { usage >&2; exit 2; }
      ((command_set == 0)) || { usage >&2; exit 2; }
      remote_command="$2"
      [[ -n "${remote_command}" ]] || { printf 'Remote command cannot be empty.\n' >&2; exit 2; }
      command_set=1
      shift 2
      ;;
    --command=*)
      ((command_set == 0)) || { usage >&2; exit 2; }
      remote_command="${1#*=}"
      [[ -n "${remote_command}" ]] || { printf 'Remote command cannot be empty.\n' >&2; exit 2; }
      command_set=1
      shift
      ;;
    --ssh-flag)
      (($# >= 2)) || { usage >&2; exit 2; }
      ssh_flags+=("$2")
      shift 2
      ;;
    --ssh-flag=*)
      ssh_flags+=("${1#*=}")
      shift
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

case "${machine}" in
  app)
    instance_output="app_instance_name"
    zone_output="zone"
    ;;
  database)
    instance_output="database_instance_name"
    zone_output="zone"
    ;;
  model)
    instance_output="model_instance_name"
    zone_output="model_zone"
    ;;
  *)
    printf 'Unknown machine role: %s\n' "${machine}" >&2
    usage >&2
    exit 2
    ;;
esac

if ! command -v gcloud >/dev/null 2>&1; then
  printf 'Required command not found: gcloud\n' >&2
  exit 2
fi

if ! project_id="$(bash "${SCRIPT_DIR}/gcp-context.sh" project-id)"; then
  exit 1
fi
instance=""
zone=""
# Terraform outputs are optional; CI can use the gleaned instance name and zone below.
if command -v terraform >/dev/null 2>&1; then
  if output="$(terraform -chdir="${TF_DIR}" output -raw "${instance_output}" 2>/dev/null)" && [[ -n "${output}" ]]; then
    instance="${output}"
  fi
  if output="$(terraform -chdir="${TF_DIR}" output -raw "${zone_output}" 2>/dev/null)" && [[ -n "${output}" ]]; then
    zone="${output}"
  fi
fi

if [[ -z "${instance}" || -z "${zone}" ]]; then
  resource_name="${MOVIEREC_RESOURCE_NAME:-}"
  configured_zone="${GCP_ZONE:-}"
  if [[ "${machine}" == model ]]; then
    model_zone="${MODEL_ZONE:-}"
    if [[ -z "${model_zone}" ]]; then
      model_zone="$(python3 "${SCRIPT_DIR}/gcp-config.py" get-tfvar model_zone 2>/dev/null || true)"
    fi
    if [[ -n "${model_zone}" && "${model_zone}" != null ]]; then
      configured_zone="${model_zone}"
    fi
  fi

  expected_instance="${resource_name}-${machine}"
  if [[ -z "${resource_name}" || -z "${configured_zone}" ]]; then
    printf 'Terraform outputs %s and %s are unavailable, and the configured VM name or zone is missing. Run make glean-vars and retry.\n' \
      "${instance_output}" "${zone_output}" >&2
    exit 1
  fi

  # Use any output that is available, then use gleaned settings for the missing value.
  instance="${instance:-${expected_instance}}"
  zone="${zone:-${configured_zone}}"
  if ! existing_instance="$(gcloud compute instances describe "${instance}" \
    --project="${project_id}" --zone="${zone}" --format='value(name)' --quiet)" || [[ "${existing_instance}" != "${instance}" ]]; then
    printf 'Terraform outputs are unavailable and GCP could not confirm VM %s in project %s, zone %s. For a fresh deployment, review make infra-plan and run make infra-apply.\n' \
      "${instance}" "${project_id}" "${zone}" >&2
    exit 1
  fi
fi

gcloud_args=(
  compute ssh "${instance}"
  --project="${project_id}"
  --zone="${zone}"
  --tunnel-through-iap
  --quiet
)
if ((command_set)); then
  gcloud_args+=(--command="${remote_command}")
fi
for ssh_flag in "${ssh_flags[@]}"; do
  gcloud_args+=(--ssh-flag="${ssh_flag}")
done

exec gcloud "${gcloud_args[@]}"
