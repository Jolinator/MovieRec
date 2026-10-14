#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
ZONE="${GCP_ZONE:-}"
RESOURCE_NAME="${MOVIEREC_RESOURCE_NAME:-}"

if [[ -z "${ZONE}" || -z "${RESOURCE_NAME}" ]]; then
  printf 'Set GCP_ZONE and MOVIEREC_RESOURCE_NAME before installing the GPU runtime.\n' >&2
  exit 2
fi
PROJECT_ID="$(bash "${SCRIPT_DIR}/gcp-context.sh" project-id)"

for command in gcloud python3; do
  command -v "${command}" >/dev/null 2>&1 || { printf 'Required command not found: %s\n' "${command}" >&2; exit 1; }
done
MODEL="${RESOURCE_NAME}-model"
mapfile -t MODEL_ZONES < <(
  gcloud compute instances list \
    --project="${PROJECT_ID}" \
    --filter="name=${MODEL}" \
    --format='value(zone)' \
    --quiet | awk -F/ 'NF { print $NF }'
)
if [[ "${#MODEL_ZONES[@]}" -ne 1 ]]; then
  printf 'Expected one model VM named %s, found %s.\n' "${MODEL}" "${#MODEL_ZONES[@]}" >&2
  exit 2
fi
MODEL_ZONE="${MODEL_ZONES[0]}"
inventory_json="$(python3 "${REPO_ROOT}/deploy/ansible/inventory.py" --list)"
GPU_ENABLED="$(printf '%s\n' "${inventory_json}" | python3 -c 'import json, sys; print("true" if json.load(sys.stdin)["all"]["vars"]["model_gpu_enabled"] else "false")')"
if [[ "${GPU_ENABLED}" != "true" ]]; then
  printf 'The model VM metadata reports GPU disabled. Enable GPU hardware before installing the NVIDIA runtime.\n' >&2
  exit 2
fi

printf 'Installing NVIDIA drivers and the container toolkit on %s. A reboot may be required.\n' "${MODEL}"
gcloud compute ssh "${MODEL}" --project="${PROJECT_ID}" --zone="${MODEL_ZONE}" --tunnel-through-iap --quiet --command='sudo bash -s' <<'REMOTE'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
GPU_RUNTIME_MARKER=/var/lib/rancher/k3s/agent/.movierec-gpu-runtime-ready
if [[ -f "${GPU_RUNTIME_MARKER}" ]] && command -v nvidia-smi >/dev/null 2>&1 && \
  nvidia-smi >/dev/null 2>&1 && command -v nvidia-ctk >/dev/null 2>&1; then
  echo 'NVIDIA drivers and container toolkit are already ready.'
  exit 0
fi

apt-get update
apt-get install -y ca-certificates curl gnupg ubuntu-drivers-common
ubuntu-drivers install

if ! command -v nvidia-ctk >/dev/null 2>&1; then
  distribution="$(. /etc/os-release; echo "${ID}${VERSION_ID}")"
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
    gpg --batch --yes --dearmor --output /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -fsSL "https://nvidia.github.io/libnvidia-container/${distribution}/libnvidia-container.list" | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
    > /etc/apt/sources.list.d/nvidia-container-toolkit.list
  apt-get update
  apt-get install -y nvidia-container-toolkit
fi

# K3s detects the NVIDIA runtime when the agent starts, so restart it.
if systemctl list-unit-files k3s-agent.service 2>/dev/null | grep -q '^k3s-agent.service'; then
  systemctl restart k3s-agent
else
  echo 'K3s agent is not installed; run make cluster-bootstrap before GPU setup.' >&2
  exit 1
fi

if ! command -v nvidia-smi >/dev/null 2>&1 || ! nvidia-smi; then
  echo 'NVIDIA driver is not ready. Reboot the model VM, then rerun make cluster-bootstrap.' >&2
  exit 1
fi
touch "${GPU_RUNTIME_MARKER}"
REMOTE
