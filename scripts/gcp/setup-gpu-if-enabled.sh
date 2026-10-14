#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
INVENTORY="${REPO_ROOT}/deploy/ansible/inventory.py"
GPU_MODE="${1:-auto}"

if (($# != 1)); then
  printf 'Usage: %s {auto|on|off}\n' "${0##*/}" >&2
  exit 2
fi
case "${GPU_MODE}" in
  auto|on|off) ;;
  *)
    printf 'Invalid GPU mode: %s (expected auto, on, or off)\n' "${GPU_MODE}" >&2
    exit 2
    ;;
esac

if [[ "${GPU_MODE}" == "off" ]]; then
  printf 'GPU mode is off; skipping NVIDIA runtime and device-plugin setup.\n'
  exit 0
fi

if ! command -v python3 >/dev/null 2>&1; then
  printf 'Required command not found: python3\n' >&2
  exit 2
fi

inventory_json="$(python3 "${INVENTORY}" --list)"
gpu_enabled="$(printf '%s\n' "${inventory_json}" | python3 -c 'import json, sys; print("true" if json.load(sys.stdin)["all"]["vars"]["model_gpu_enabled"] else "false")')"
if [[ "${gpu_enabled}" != "true" ]]; then
  if [[ "${GPU_MODE}" == "on" ]]; then
    printf 'GPU=on was requested, but the model VM metadata says GPU is disabled. Enable GPU hardware before continuing.\n' >&2
    exit 1
  fi
  printf 'The model VM has GPU disabled; skipping NVIDIA runtime and device-plugin setup.\n'
  exit 0
fi

printf 'GPU is enabled on the model VM; preparing the NVIDIA runtime and Kubernetes device plugin.\n'
bash "${SCRIPT_DIR}/install-gpu-runtime.sh"
make -C "${REPO_ROOT}" gpu-setup
