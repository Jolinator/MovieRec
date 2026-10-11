#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
KUBECONFIG_PATH="${KUBECONFIG:-${REPO_ROOT}/kubeconfig}"

for command in gcloud terraform; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

INSTANCE="$(terraform -chdir="${TF_DIR}" output -raw instance_name)"
ZONE="$(terraform -chdir="${TF_DIR}" output -raw zone)"
if [[ -z "${INSTANCE}" || -z "${ZONE}" ]]; then
  printf 'Terraform outputs instance_name and zone must be available.\n' >&2
  exit 1
fi

mkdir -p "$(dirname -- "${KUBECONFIG_PATH}")"
TEMP_KUBECONFIG="$(mktemp "${KUBECONFIG_PATH}.XXXXXX")"
trap 'rm -f -- "${TEMP_KUBECONFIG}"' EXIT

printf 'Downloading the K3s kubeconfig from %s (%s)...\n' "${INSTANCE}" "${ZONE}"
gcloud compute ssh "${INSTANCE}" --zone="${ZONE}" --tunnel-through-iap \
  --command='sudo cat /etc/rancher/k3s/k3s.yaml' > "${TEMP_KUBECONFIG}"
chmod 600 "${TEMP_KUBECONFIG}"
mv -- "${TEMP_KUBECONFIG}" "${KUBECONFIG_PATH}"
trap - EXIT

printf 'Kubeconfig saved at %s\n' "${KUBECONFIG_PATH}"
printf 'In another Cloud Shell terminal, run:\n'
printf 'export KUBECONFIG=%q\n' "${KUBECONFIG_PATH}"
printf 'kubectl get nodes\n\n'
printf 'Keep this terminal open while using kubectl. Press Ctrl+C to close the tunnel.\n'

exec gcloud compute ssh "${INSTANCE}" --zone="${ZONE}" --tunnel-through-iap \
  -- -N -L 6443:127.0.0.1:6443
