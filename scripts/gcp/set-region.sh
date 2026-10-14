#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
GCP_CONFIG="${SCRIPT_DIR}/gcp-config.py"
TFVARS="${REPO_ROOT}/deploy/gcp/terraform/terraform.tfvars"
SERVER_ENV="${REPO_ROOT}/deploy/gcp/secrets/server.env"

usage() {
  printf 'Usage: %s --region REGION --zone ZONE [--model-zone ZONE]\n' "${0##*/}"
}

region=""
zone=""
model_zone=""

while (($#)); do
  case "$1" in
    --region)
      (($# >= 2)) || { usage >&2; exit 2; }
      region="$2"
      shift 2
      ;;
    --zone)
      (($# >= 2)) || { usage >&2; exit 2; }
      zone="$2"
      shift 2
      ;;
    --model-zone)
      (($# >= 2)) || { usage >&2; exit 2; }
      model_zone="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${region}" || -z "${zone}" ]]; then
  usage >&2
  exit 2
fi

if [[ ! "${region}" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
  printf 'Invalid region name: %s\n' "${region}" >&2
  exit 2
fi

if [[ ! "${zone}" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ || "${zone}" != "${region}-"* ]]; then
  printf 'Zone %s must belong to region %s.\n' "${zone}" "${region}" >&2
  exit 2
fi

if [[ -z "${model_zone}" ]]; then
  model_zone="${zone}"
elif [[ ! "${model_zone}" =~ ^[a-z0-9]+(-[a-z0-9]+)+$ ]]; then
  printf 'Invalid model zone name: %s\n' "${model_zone}" >&2
  exit 2
fi

if [[ ! -f "${TFVARS}" || ! -f "${SERVER_ENV}" ]]; then
  printf 'Initialize the deployment config first with make secrets-init and make infra-config.\n' >&2
  exit 1
fi

python3 "${GCP_CONFIG}" set-server-env GCP_REGION "${region}"
python3 "${GCP_CONFIG}" set-server-env GCP_ZONE "${zone}"
python3 "${GCP_CONFIG}" set-tfvar region "${region}"
python3 "${GCP_CONFIG}" set-tfvar zone "${zone}"
python3 "${GCP_CONFIG}" set-tfvar model_zone "${model_zone}"

printf 'Updated app/database location to %s / %s and model zone to %s.\n' \
  "${region}" "${zone}" "${model_zone}"
printf 'Configuration only: review make infra-plan before running make infra-apply.\n'
