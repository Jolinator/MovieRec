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
if [[ -z "${PROJECT_ID}" || "${PROJECT_ID}" == "(unset)" ]]; then
  printf 'No active Google Cloud project is configured.\n' >&2
  printf 'Select one with: gcloud config set project PROJECT_ID\n' >&2
  exit 1
fi

if [[ ! -e "${TFVARS}" ]]; then
  cp "${TF_DIR}/terraform.tfvars.example" "${TFVARS}"
  printf 'Created %s from the example.\n' "${TFVARS}"
fi

chmod 600 "${TFVARS}"
sed -i "s/^project_id[[:space:]]*=.*/project_id = \"${PROJECT_ID}\"/" "${TFVARS}"

if ! grep -q '^project_id[[:space:]]*=' "${TFVARS}"; then
  printf 'terraform.tfvars does not contain a project_id setting: %s\n' "${TFVARS}" >&2
  exit 1
fi

printf 'Terraform project_id set to %s in %s\n' "${PROJECT_ID}" "${TFVARS}"
printf 'Review the remaining Terraform settings before running make infra-plan.\n'
