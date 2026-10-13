#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
TFVARS="${TF_DIR}/terraform.tfvars"
GPU_MACHINE_TYPE="${1:-g2-standard-4}"

if [[ ! "${GPU_MACHINE_TYPE}" =~ ^g2-[a-z0-9-]+$ ]]; then
  printf 'GPU machine type must be a G2 type, for example g2-standard-4.\n' >&2
  exit 2
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
    sed -i -E "s|^[[:space:]]*${key}[[:space:]]*=.*|${key} = ${value}|" "${TFVARS}"
  else
    printf '%s = %s\n' "${key}" "${value}" >> "${TFVARS}"
  fi
}

set_tfvar model_machine_type "\"${GPU_MACHINE_TYPE}\""
set_tfvar model_gpu_enabled true

printf 'Enabled GPU settings in %s: model_machine_type=%s, model_gpu_enabled=true\n' \
  "${TFVARS}" "${GPU_MACHINE_TYPE}"
printf '%s\n' 'Review the model VM replacement with make gpu-infra-plan, then run make gpu-infra-apply before make gpu-runtime.'
