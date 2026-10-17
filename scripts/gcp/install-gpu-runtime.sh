#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"

for command in gcloud python3 terraform; do
  command -v "${command}" >/dev/null 2>&1 || { printf 'Required command not found: %s\n' "${command}" >&2; exit 1; }
done
MODEL="model VM"
inventory_json="$(python3 "${REPO_ROOT}/deploy/ansible/inventory.py" --list)"
GPU_ENABLED="$(printf '%s\n' "${inventory_json}" | python3 -c 'import json, sys; print("true" if json.load(sys.stdin)["all"]["vars"]["model_gpu_enabled"] else "false")')"
if [[ "${GPU_ENABLED}" != "true" ]]; then
  printf 'The model VM metadata reports GPU disabled. Enable GPU hardware before installing the NVIDIA runtime.\n' >&2
  exit 2
fi

printf 'Installing NVIDIA drivers and the container toolkit on %s.\n' "${MODEL}"
model_ssh() {
  bash "${SCRIPT_DIR}/ssh-machine.sh" model "$@"
}

remote_status=0
if model_ssh --command='sudo bash -s' <<'REMOTE'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
GPU_RUNTIME_MARKER=/var/lib/rancher/k3s/agent/.movierec-gpu-runtime-ready
if [[ -f "${GPU_RUNTIME_MARKER}" ]] && command -v nvidia-smi >/dev/null 2>&1 && \
  nvidia-smi >/dev/null 2>&1 && command -v nvidia-ctk >/dev/null 2>&1 && \
  command -v nvidia-container-runtime >/dev/null 2>&1 && \
  grep -q nvidia /var/lib/rancher/k3s/agent/etc/containerd/config.toml; then
  echo 'NVIDIA drivers and container toolkit are already ready.'
  exit 0
fi

apt-get update
apt-get install -y ca-certificates curl gnupg ubuntu-drivers-common
ubuntu-drivers install

if ! command -v nvidia-ctk >/dev/null 2>&1; then
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
    gpg --batch --yes --dearmor --output /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -fsSL https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
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

if ! command -v nvidia-container-runtime >/dev/null 2>&1; then
  echo 'NVIDIA container runtime was not installed.' >&2
  exit 1
fi
if ! grep -q nvidia /var/lib/rancher/k3s/agent/etc/containerd/config.toml; then
  echo 'K3s did not detect the NVIDIA runtime in its containerd configuration.' >&2
  exit 1
fi

if ! command -v nvidia-smi >/dev/null 2>&1 || ! nvidia-smi; then
  echo 'MOVIDREC_GPU_REBOOT_REQUIRED'
  exit 42
fi
touch "${GPU_RUNTIME_MARKER}"
REMOTE
then
  :
else
  remote_status=$?
fi

if [[ "${remote_status}" -eq 42 ]]; then
  printf 'NVIDIA driver needs a reboot; rebooting %s and waiting for it to return.\n' "${MODEL}"
  bash "${SCRIPT_DIR}/reboot-machine.sh" model \
    --wait-command 'sudo nvidia-smi >/dev/null 2>&1' \
    --timeout-seconds 600

  model_ssh --command='sudo bash -s' <<'VERIFY'
set -euo pipefail
GPU_RUNTIME_MARKER=/var/lib/rancher/k3s/agent/.movierec-gpu-runtime-ready
nvidia-smi
command -v nvidia-ctk >/dev/null
command -v nvidia-container-runtime >/dev/null
grep -q nvidia /var/lib/rancher/k3s/agent/etc/containerd/config.toml
touch "${GPU_RUNTIME_MARKER}"
VERIFY
elif [[ "${remote_status}" -ne 0 ]]; then
  exit "${remote_status}"
fi
