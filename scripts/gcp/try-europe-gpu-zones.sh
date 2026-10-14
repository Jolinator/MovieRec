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

cat <<'NOTICE'
This checks supported European T4/L4 zones by planning and applying one model VM at a time.
It stops after the first successful apply. The app and database VMs are not Terraform targets.
Moving the model between zones can replace its zonal model-data disk and remove cached files.
The successful model VM remains running and billable until you stop it.
NOTICE
read -r -p 'Type TRY to continue: ' confirmation
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
trap 'rm -f "${LOG_FILE}"' EXIT
results=()

print_results() {
  printf '\nAttempt summary (zones after a successful allocation were not tested):\n'
  printf '  %s\n' "${results[@]}"
}

for candidate in "${candidates[@]}"; do
  read -r zone gpu_type <<<"${candidate}"
  printf '\n===== Trying %s with %s =====\n' "${zone}" "${gpu_type}"

  # Keep the app/database zone and region unchanged. This is a separate model location.
  python3 "${GCP_CONFIG}" set-tfvar model_zone "${zone}"

  if ! make infra-plan MODEL_ONLY=1 GPU=on GPU_TYPE="${gpu_type}"; then
    results+=("${zone} ${gpu_type}: PLAN ERROR; capacity not tested")
    print_results
    printf 'Plan failed for %s / %s; stopping so the configuration error can be fixed.\n' \
      "${zone}" "${gpu_type}" >&2
    exit 1
  fi

  : >"${LOG_FILE}"
  set +e
  make infra-apply MODEL_ONLY=1 GPU=auto 2>&1 | tee "${LOG_FILE}"
  apply_status=${PIPESTATUS[0]}
  set -e

  if [[ "${apply_status}" -eq 0 ]]; then
    results+=("${zone} ${gpu_type}: CREATED; capacity confirmed at this time")
    print_results
    printf '\nGPU VM created successfully: %s in %s.\n' "${gpu_type}" "${zone}"
    printf 'The script stopped after the first success. Stop the model VM when you are done to avoid GPU runtime charges.\n'
    exit 0
  fi

  if grep -Eiq 'GPUS_ALL_REGIONS|gpus_all_regions|GPUS-ALL-REGIONS|limit: 0[^[:cntrl:]]*globally' "${LOG_FILE}"; then
    results+=("${zone} ${gpu_type}: GLOBAL GPU QUOTA ERROR; capacity not determined")
    print_results
    printf '\nThe global GPU quota blocked this request; changing zones will not fix that. Request quota, then rerun.\n' >&2
    exit 1
  fi

  if grep -Eiq 'quota[^[:cntrl:]]*exceeded|quota[^[:cntrl:]]*limit' "${LOG_FILE}"; then
    results+=("${zone} ${gpu_type}: OTHER QUOTA ERROR; capacity not determined")
    printf '\nQuota blocked %s / %s; trying the next candidate because quota may differ by region or GPU family.\n' \
      "${zone}" "${gpu_type}"
    continue
  fi

  if grep -Eiq 'STOCKOUT|ZONE_RESOURCE_POOL_EXHAUSTED|RESOURCE_POOL_EXHAUSTED|does not have enough resources|currently unavailable|resource pool exhausted' "${LOG_FILE}"; then
    results+=("${zone} ${gpu_type}: STOCKOUT; unavailable at this time")
    printf '\nNo capacity for %s / %s right now; trying the next listed zone.\n' "${zone}" "${gpu_type}"
    continue
  fi

  results+=("${zone} ${gpu_type}: OTHER ERROR; capacity not determined")
  print_results
  printf '\nApply failed for a reason other than a recognized capacity error; stopping. See the error above.\n' >&2
  exit "${apply_status}"
done

print_results
printf '\nNo listed European T4/L4 candidate succeeded. Terraform variables retain the last attempted model zone/type.\n' >&2
exit 3
