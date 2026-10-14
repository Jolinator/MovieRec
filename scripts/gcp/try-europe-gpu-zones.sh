#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TFVARS="${REPO_ROOT}/deploy/gcp/terraform/terraform.tfvars"
GCP_CONFIG="${SCRIPT_DIR}/gcp-config.py"

if [[ ! -f "${TFVARS}" ]]; then
  printf 'Missing %s. Run make infra-config first, then rerun this script.\n' "${TFVARS}" >&2
  exit 2
fi

for command in make python3 terraform gcloud; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 2
  fi
done

cd "${REPO_ROOT}"

read -r -p 'Type TRY to confirm the model-only European GPU trials: ' confirmation
if [[ "${confirmation}" != "TRY" ]]; then
  printf 'Cancelled.\n'
  exit 0
fi

# These are the European zones currently documented for N1+T4 and G2/L4.
# Keep each GPU type only in zones where Google lists that accelerator family.
candidates=(
  "europe-west4-b nvidia-l4"
  "europe-west4-b nvidia-tesla-t4"
  "europe-west4-a nvidia-l4"
  "europe-west4-a nvidia-tesla-t4"
  "europe-west4-c nvidia-l4"
  "europe-west4-c nvidia-tesla-t4"
  "europe-west1-b nvidia-l4"
  "europe-west1-b nvidia-tesla-t4"
  "europe-west1-c nvidia-l4"
  "europe-west1-c nvidia-tesla-t4"
  "europe-west1-d nvidia-tesla-t4"
  "europe-west2-a nvidia-l4"
  "europe-west2-a nvidia-tesla-t4"
  "europe-west2-b nvidia-l4"
  "europe-west2-b nvidia-tesla-t4"
  "europe-west3-a nvidia-l4"
  "europe-west3-b nvidia-l4"
  "europe-west3-b nvidia-tesla-t4"
  "europe-west6-b nvidia-l4"
  "europe-west6-c nvidia-l4"
  "europe-central2-b nvidia-tesla-t4"
  "europe-central2-c nvidia-tesla-t4"
)

LOG_FILE="$(mktemp)"
ERROR_FILE="$(mktemp)"
trap 'rm -f "${LOG_FILE}" "${ERROR_FILE}"' EXIT

print_errors() {
  local log_file="$1"
  : >"${ERROR_FILE}"
  awk '
    /^[[:space:]]*\342\225\267/ { in_block = 1; block = $0 ORS; next }
    in_block {
      block = block $0 ORS
      if (/^[[:space:]]*\342\225\265/) {
        if (block ~ /Error:/) printf "%s", block
        in_block = 0
        block = ""
      }
      next
    }
    END {
      if (in_block && block ~ /Error:/) printf "%s", block
    }
  ' "${log_file}" >"${ERROR_FILE}"

  if [[ -s "${ERROR_FILE}" ]]; then
    cat "${ERROR_FILE}"
  else
    grep -Eai 'error|failed|failure|denied|quota|stockout|unavailable' "${log_file}" || true
  fi
}

for candidate in "${candidates[@]}"; do
  read -r zone gpu_type <<<"${candidate}"
  printf 'Trying %s with GPU %s\n' "${zone}" "${gpu_type}"

  # Keep the app/database zone and region unchanged. This is a separate model location.
  if ! python3 "${GCP_CONFIG}" set-tfvar model_zone "${zone}" >"${LOG_FILE}" 2>&1; then
    print_errors "${LOG_FILE}"
    exit 1
  fi

  if ! make infra-plan MODEL_ONLY=1 GPU=on GPU_TYPE="${gpu_type}" >"${LOG_FILE}" 2>&1; then
    print_errors "${LOG_FILE}"
    exit 1
  fi

  : >"${LOG_FILE}"
  if make infra-apply MODEL_ONLY=1 TF_AUTO_APPROVE=1 GPU=auto >"${LOG_FILE}" 2>&1; then
    exit 0
  else
    apply_status=$?
    print_errors "${LOG_FILE}"
  fi

  if grep -Eiq 'GPUS_ALL_REGIONS|gpus_all_regions|GPUS-ALL-REGIONS|limit: 0[^[:cntrl:]]*globally' "${LOG_FILE}"; then
    exit 1
  fi

  if grep -Eiq 'quota[^[:cntrl:]]*exceeded|quota[^[:cntrl:]]*limit' "${LOG_FILE}"; then
    continue
  fi

  if grep -Eiq 'STOCKOUT|ZONE_RESOURCE_POOL_EXHAUSTED|RESOURCE_POOL_EXHAUSTED|does not have enough resources|currently unavailable|resource pool exhausted' "${LOG_FILE}"; then
    continue
  fi

  exit "${apply_status}"
done

exit 3
