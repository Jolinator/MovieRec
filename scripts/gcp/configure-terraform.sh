#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
TFVARS="${TF_DIR}/terraform.tfvars"

if ! command -v gcloud >/dev/null 2>&1; then
  printf 'Required command not found: gcloud\n' >&2
  exit 1
fi

PROJECT_ID="$(gcloud config get-value project 2>/dev/null)"
REGION="${GCP_REGION:-}"
ZONE="${GCP_ZONE:-}"
if [[ -z "${PROJECT_ID}" || "${PROJECT_ID}" == "(unset)" ]]; then
  printf 'No active Google Cloud project is configured.\n' >&2
  printf 'Select one with: gcloud config set project PROJECT_ID\n' >&2
  exit 1
fi
if [[ -z "${REGION}" || -z "${ZONE}" ]]; then
  printf 'Set GCP_REGION and GCP_ZONE before configuring Terraform.\n' >&2
  exit 1
fi

if [[ ! -e "${TFVARS}" ]]; then
  cp "${TF_DIR}/terraform.tfvars.example" "${TFVARS}"
  printf 'Created %s from the example.\n' "${TFVARS}"
fi

chmod 600 "${TFVARS}"

set_tfvar() {
  local key="$1"
  local value="$2"
  if grep -Eq "^[[:space:]]*${key}[[:space:]]*=" "${TFVARS}"; then
    sed -i -E "s|^[[:space:]]*${key}[[:space:]]*=.*|${key} = \"${value}\"|" "${TFVARS}"
  else
    printf '%s = "%s"\n' "${key}" "${value}" >> "${TFVARS}"
  fi
}

set_tfvar project_id "${PROJECT_ID}"
set_tfvar region "${REGION}"
set_tfvar zone "${ZONE}"

printf 'Terraform project_id, region, and zone set from the active GCP project and GCP_REGION/GCP_ZONE in %s\n' "${TFVARS}"
printf 'Review the remaining Terraform settings before running make infra-plan.\n'
