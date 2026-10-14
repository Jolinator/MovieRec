#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
TFVARS="${TF_DIR}/terraform.tfvars"
GCP_CONFIG="${SCRIPT_DIR}/gcp-config.py"

PROJECT_ID="$(bash "${SCRIPT_DIR}/gcp-context.sh" active-project-id)"

if [[ ! -e "${TFVARS}" ]]; then
  cp "${TF_DIR}/terraform.tfvars.example" "${TFVARS}"
  printf 'Created %s from the example.\n' "${TFVARS}"
fi

chmod 600 "${TFVARS}"

REGION="${GCP_REGION:-}"
ZONE="${GCP_ZONE:-}"
if [[ -z "${REGION}" || -z "${ZONE}" ]]; then
  printf 'GCP_REGION and GCP_ZONE must be supplied by make or the environment resolver.\n' >&2
  exit 1
fi
python3 "${GCP_CONFIG}" set-tfvar project_id "${PROJECT_ID}"
python3 "${GCP_CONFIG}" set-tfvar region "${REGION}"
python3 "${GCP_CONFIG}" set-tfvar zone "${ZONE}"

printf 'Terraform project_id set from the active GCP project; region and zone use terraform.tfvars first, then GCP_REGION/GCP_ZONE, in %s\n' "${TFVARS}"
printf 'Review the remaining Terraform settings before running make infra-plan.\n'
