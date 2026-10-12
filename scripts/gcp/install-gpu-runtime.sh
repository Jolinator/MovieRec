#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"

for command in gcloud terraform; do
  command -v "${command}" >/dev/null 2>&1 || { printf 'Required command not found: %s\n' "${command}" >&2; exit 1; }
done
ZONE="$(terraform -chdir="${TF_DIR}" output -raw zone)"
MODEL="$(terraform -chdir="${TF_DIR}" output -raw model_instance_name)"
[[ -n "${ZONE}" && -n "${MODEL}" ]] || { printf 'Terraform outputs are incomplete; run infra-apply first.\n' >&2; exit 1; }

printf 'Installing NVIDIA drivers and the container toolkit on %s. A reboot may be required.\n' "${MODEL}"
gcloud compute ssh "${MODEL}" --zone="${ZONE}" --tunnel-through-iap --quiet --command='sudo bash -s' <<'REMOTE'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl gnupg ubuntu-drivers-common
ubuntu-drivers install

if ! command -v nvidia-ctk >/dev/null 2>&1; then
  distribution="$(. /etc/os-release; echo "${ID}${VERSION_ID}")"
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
    gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -fsSL "https://nvidia.github.io/libnvidia-container/${distribution}/libnvidia-container.list" | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
    > /etc/apt/sources.list.d/nvidia-container-toolkit.list
  apt-get update
  apt-get install -y nvidia-container-toolkit
fi

CONTAINERD_TEMPLATE=/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl
if [[ -f "${CONTAINERD_TEMPLATE}" ]]; then
  nvidia-ctk runtime configure --runtime=containerd --config="${CONTAINERD_TEMPLATE}"
  systemctl restart k3s-agent || true
else
  echo 'K3s is not installed yet; run make cluster-bootstrap, then rerun make gpu-runtime.'
fi

if command -v nvidia-smi >/dev/null 2>&1; then
  nvidia-smi || echo 'nvidia-smi could not load the driver yet; reboot this VM and rerun make gpu-runtime.'
else
  echo 'Reboot the model VM to load the NVIDIA driver, then rerun make gpu-runtime.'
fi
REMOTE
