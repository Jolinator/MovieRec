#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TFVARS="${REPO_ROOT}/deploy/gcp/terraform/terraform.tfvars"
GCP_CONFIG="${SCRIPT_DIR}/gcp-config.py"

region=""
while (($#)); do
  case "$1" in
    --region)
      if (($# < 2)); then
        printf 'Missing value for --region. Choose europe or us.\n' >&2
        exit 2
      fi
      region="$2"
      shift 2
      ;;
    --region=*)
      region="${1#*=}"
      shift
      ;;
    -h|--help)
      printf 'Usage: %s --region europe|us\n' "${0##*/}"
      exit 0
      ;;
    *)
      printf 'Unknown argument: %s\nUsage: %s --region europe|us\n' "$1" "${0##*/}" >&2
      exit 2
      ;;
  esac
done

if [[ -z "${region}" ]]; then
  printf 'Set --region to europe or us.\n' >&2
  exit 2
fi

case "${region}" in
  europe)
    region_label="European"
    # These are the European zones currently documented for N1+T4 and G2/L4.
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
    ;;
  us)
    region_label="US"
    # Include each US zone documented for N1+T4 and/or G2/L4, pairing only
    # GPU types listed as supported in that specific zone.
    candidates=(
      "us-east4-a nvidia-l4"
      "us-east4-a nvidia-tesla-t4"
      "us-east4-b nvidia-tesla-t4"
      "us-east4-c nvidia-l4"
      "us-east4-c nvidia-tesla-t4"
      "us-east1-b nvidia-l4"
      "us-east1-b nvidia-tesla-t4"
      "us-east1-c nvidia-l4"
      "us-east1-c nvidia-tesla-t4"
      "us-central1-a nvidia-l4"
      "us-central1-a nvidia-tesla-t4"
      "us-central1-b nvidia-l4"
      "us-central1-b nvidia-tesla-t4"
      "us-central1-c nvidia-l4"
      "us-central1-c nvidia-tesla-t4"
      "us-central1-f nvidia-tesla-t4"
      "us-west1-a nvidia-l4"
      "us-west1-a nvidia-tesla-t4"
      "us-west1-b nvidia-l4"
      "us-west1-b nvidia-tesla-t4"
      "us-west2-b nvidia-tesla-t4"
      "us-west2-c nvidia-tesla-t4"
      "us-west3-b nvidia-tesla-t4"
      "us-west4-a nvidia-l4"
      "us-west4-a nvidia-tesla-t4"
      "us-west4-b nvidia-tesla-t4"
      "us-west4-c nvidia-l4"
    )
    ;;
  *)
    printf 'Unsupported region "%s". Choose europe or us.\n' "${region}" >&2
    exit 2
    ;;
esac

if [[ ! -f "${TFVARS}" ]]; then
  printf 'Missing %s. Run make glean-vars first, then rerun this script.\n' "${TFVARS}" >&2
  exit 2
fi

for command in make python3 terraform gcloud; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 2
  fi
done

cd "${REPO_ROOT}"

read -r -p "Type TRY to confirm the model-only ${region_label} GPU trials: " confirmation
if [[ "${confirmation}" != "TRY" ]]; then
  printf 'Cancelled.\n'
  exit 0
fi

LOG_FILE="$(mktemp)"
trap 'rm -f "${LOG_FILE}"' EXIT

print_errors() {
  local log_file="$1"
  grep -Eai 'Error:|currently unavailable|ZONE_RESOURCE_POOL_EXHAUSTED|STOCKOUT|resource pool exhausted|does not have enough resources|quota|permission denied|access denied|not found|invalid' "${log_file}" \
    | grep -Ev '^make(\[[0-9]+\])?: \*\*\*' || true
}

for candidate in "${candidates[@]}"; do
  read -r zone gpu_type <<<"${candidate}"
  printf 'Trying %s with GPU %s\n' "${zone}" "${gpu_type}"

  # Keep the app/database zone and region unchanged. This is a separate model location.
  if ! python3 "${GCP_CONFIG}" set-tfvar model_zone "${zone}" >"${LOG_FILE}" 2>&1; then
    print_errors "${LOG_FILE}"
    exit 1
  fi

  # Capacity trials use Terraform's machine-type default for each GPU family.
  # Clear overrides such as n1-standard-* that are incompatible with L4, and
  # keep GPU_COUNT at one because only N1+T4 supports multi-GPU counts.
  if ! make infra-plan MODEL_ONLY=1 GPU=on GPU_TYPE="${gpu_type}" GPU_MACHINE_TYPE= GPU_COUNT=1 >"${LOG_FILE}" 2>&1; then
    print_errors "${LOG_FILE}"
    exit 1
  fi

  : >"${LOG_FILE}"
  if make infra-apply MODEL_ONLY=1 TF_AUTO_APPROVE=1 GPU=auto GPU_MACHINE_TYPE= GPU_COUNT=1 >"${LOG_FILE}" 2>&1; then
    printf 'Success: GPU VM created in %s with %s\n' "${zone}" "${gpu_type}"
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
