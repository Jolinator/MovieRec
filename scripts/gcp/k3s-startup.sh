#!/bin/bash
set -euo pipefail
exec > >(tee -a /var/log/movierec-bootstrap.log) 2>&1

retry_forever() {
  local description="$1"
  shift
  local delay=2

  until "$@"; do
    echo "${description} failed; retrying in ${delay}s" >&2
    sleep "${delay}"
    if (( delay < 30 )); then
      delay=$((delay * 2))
      (( delay > 30 )) && delay=30
    fi
  done
}

metadata() {
  retry_forever "Reading metadata attribute $1" \
    curl -fsS -H 'Metadata-Flavor: Google' \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1"
}

export DEBIAN_FRONTEND=noninteractive
retry_forever "apt update" apt-get -o DPkg::Lock::Timeout=300 update
retry_forever "Installing bootstrap packages" \
  apt-get -o DPkg::Lock::Timeout=300 install -y ca-certificates curl e2fsprogs

ROLE="$(metadata movierec_role)"
NODE_NAME="$(metadata node_name)"
NODE_LABEL="$(metadata node_label)"
DATA_DISK_DEVICE="$(metadata data_disk_device)"
GPU_ENABLED="$(metadata gpu_enabled)"

if [[ ! "${ROLE}" =~ ^(app|database|model)$ ]]; then
  echo "Unsupported movierec_role: ${ROLE}" >&2
  exit 1
fi

# local-path provisions PVCs under this directory. The disk is attached by
# Terraform before boot, so this mount is ready before K3s starts.
MOUNT_POINT=/var/lib/rancher/k3s/storage
mkdir -p "${MOUNT_POINT}"
attempt=0
until [[ -b "${DATA_DISK_DEVICE}" ]]; do
  attempt=$((attempt + 1))
  if (( attempt % 15 == 1 )); then
    echo "Waiting for data disk ${DATA_DISK_DEVICE} to appear"
  fi
  sleep 2
done

if ! blkid "${DATA_DISK_DEVICE}" >/dev/null 2>&1; then
  echo "Formatting new discardable data disk ${DATA_DISK_DEVICE}"
  mkfs.ext4 -F "${DATA_DISK_DEVICE}"
fi
DATA_DISK_UUID="$(blkid -s UUID -o value "${DATA_DISK_DEVICE}")"
FSTAB_LINE="UUID=${DATA_DISK_UUID} ${MOUNT_POINT} ext4 defaults,nofail 0 2"
if ! grep -Fqx "${FSTAB_LINE}" /etc/fstab; then
  printf '%s\n' "${FSTAB_LINE}" >> /etc/fstab
fi
if ! mountpoint -q "${MOUNT_POINT}"; then
  retry_forever "Mounting data disk at ${MOUNT_POINT}" mount "${MOUNT_POINT}"
fi

# This helper is invoked by cluster-bootstrap after the server has generated
# its one-time join token. The token is read from stdin, never put in metadata
# or a command line, and removed from memory after the installer exits.
cat > /usr/local/sbin/movierec-join-k3s-agent <<'JOIN'
#!/bin/bash
set -euo pipefail

if [[ "${1:-}" == "" || "${2:-}" == "" || "${3:-}" == "" || "${4:-}" == "" ]]; then
  echo "usage: movierec-join-k3s-agent SERVER_URL NODE_NAME NODE_LABEL K3S_VERSION" >&2
  exit 2
fi
retry_forever() {
  local description="$1"
  shift
  local delay=2

  until "$@"; do
    echo "${description} failed; retrying in ${delay}s" >&2
    sleep "${delay}"
    if (( delay < 30 )); then
      delay=$((delay * 2))
      (( delay > 30 )) && delay=30
    fi
  done
}

SERVER_URL="$1"
NODE_NAME="$2"
NODE_LABEL="$3"
K3S_VERSION="$4"
if [[ -f /etc/systemd/system/k3s-agent.service ]] || systemctl list-unit-files k3s-agent.service 2>/dev/null | grep -q '^k3s-agent.service'; then
  retry_forever "Starting K3s agent" systemctl start k3s-agent
  exit 0
fi

IFS= read -r K3S_TOKEN || true
if [[ -z "${K3S_TOKEN}" ]]; then
  echo "No K3s token was supplied on stdin" >&2
  exit 1
fi

# Do not enable xtrace around the token-bearing installer invocation.
export K3S_TOKEN
install_k3s_agent() {
  curl -sfL https://get.k3s.io | \
    INSTALL_K3S_VERSION="${K3S_VERSION}" \
    K3S_URL="${SERVER_URL}" \
    K3S_TOKEN="${K3S_TOKEN}" \
    INSTALL_K3S_EXEC="agent --node-name ${NODE_NAME} --node-label ${NODE_LABEL}" \
    sh -s -
}
retry_forever "Installing K3s agent" install_k3s_agent
unset K3S_TOKEN
JOIN
chmod 700 /usr/local/sbin/movierec-join-k3s-agent

if [[ "${ROLE}" == "app" ]]; then
  PUBLIC_IP="$(metadata k3s_tls_san)"
  PRIVATE_IP="$(hostname -I | awk '{print $1}')"
  if command -v k3s >/dev/null 2>&1; then
    retry_forever "Starting K3s server" systemctl start k3s
  else
    install_k3s_server() {
      curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable sh -s - server \
        --secrets-encryption \
        --tls-san "${PUBLIC_IP}" \
        --tls-san "${PRIVATE_IP}" \
        --node-name "${NODE_NAME}" \
        --node-label "${NODE_LABEL}"
    }
    retry_forever "Installing K3s server" install_k3s_server
    retry_forever "Starting K3s server" systemctl start k3s
  fi

  until kubectl --request-timeout=10s get nodes >/dev/null 2>&1; do sleep 5; done
  until kubectl --request-timeout=10s wait --for=condition=Ready \
    "node/${NODE_NAME}" --timeout=30s; do
    echo "K3s node ${NODE_NAME} is not Ready yet; continuing to wait"
    sleep 5
  done
  retry_forever "Labeling K3s node ${NODE_NAME}" \
    kubectl --request-timeout=10s label node "${NODE_NAME}" "${NODE_LABEL}" --overwrite
  retry_forever "Reading K3s nodes" \
    kubectl --request-timeout=10s get nodes --show-labels
else
  echo "${ROLE} VM is prepared; run make cluster-bootstrap to join it to ${NODE_NAME} server."
  if [[ "${GPU_ENABLED}" == "true" ]]; then
    echo "GPU attached. Install the NVIDIA driver/runtime before enabling the GPU manifest."
  fi
fi
