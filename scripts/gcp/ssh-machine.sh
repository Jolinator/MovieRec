#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"

usage() {
  printf 'Usage: %s {app|database|model} [--command COMMAND]\n' "${0##*/}"
  printf 'Without --command, opens an interactive SSH session.\n'
}

if (($# == 0)); then
  usage >&2
  exit 2
fi

machine="$1"
shift
remote_command=""
if (($#)); then
  if (($# != 2)) || [[ "$1" != "--command" ]]; then
    usage >&2
    exit 2
  fi
  remote_command="$2"
  if [[ -z "${remote_command}" ]]; then
    printf 'Remote command cannot be empty.\n' >&2
    exit 2
  fi
fi

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

for command in terraform gcloud; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 2
  fi
done

if ! project_id="$(bash "${SCRIPT_DIR}/gcp-context.sh" project-id)"; then
  exit 1
fi
if ! instance="$(terraform -chdir="${TF_DIR}" output -raw "${instance_output}")" || [[ -z "${instance}" ]]; then
  printf 'Could not read Terraform output %s from %s. Initialize/apply Terraform first.\n' \
    "${instance_output}" "${TF_DIR}" >&2
  exit 1
fi
if ! zone="$(terraform -chdir="${TF_DIR}" output -raw "${zone_output}")" || [[ -z "${zone}" ]]; then
  printf 'Could not read Terraform output %s from %s. Initialize/apply Terraform first.\n' \
    "${zone_output}" "${TF_DIR}" >&2
  exit 1
fi

gcloud_args=(
  compute ssh "${instance}"
  --project="${project_id}"
  --zone="${zone}"
  --tunnel-through-iap
  --quiet
)
if [[ -n "${remote_command}" ]]; then
  gcloud_args+=(--command="${remote_command}")
fi

exec gcloud "${gcloud_args[@]}"
