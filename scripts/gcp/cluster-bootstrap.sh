#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"

for command in gcloud terraform; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

ZONE="$(terraform -chdir="${TF_DIR}" output -raw zone)"
APP="$(terraform -chdir="${TF_DIR}" output -raw app_instance_name)"
DB="$(terraform -chdir="${TF_DIR}" output -raw database_instance_name)"
MODEL="$(terraform -chdir="${TF_DIR}" output -raw model_instance_name)"
APP_PRIVATE_IP="$(terraform -chdir="${TF_DIR}" output -raw app_private_ip)"
for value in "${ZONE}" "${APP}" "${DB}" "${MODEL}" "${APP_PRIVATE_IP}"; do
  [[ -n "${value}" ]] || { printf 'Terraform outputs are incomplete; run infra-apply first.\n' >&2; exit 1; }
done

printf 'Waiting for the app/control-plane node to finish K3s startup...\n'
APP_READY=0
for attempt in {1..60}; do
  if gcloud compute ssh "${APP}" --zone="${ZONE}" --tunnel-through-iap --quiet \
    --command='sudo systemctl is-active --quiet k3s && sudo test -s /var/lib/rancher/k3s/server/node-token'; then
    APP_READY=1
    break
  fi
  sleep 10
done
if [[ "${APP_READY}" != 1 ]]; then
  printf 'App K3s server did not finish startup within 10 minutes.\n' >&2
  exit 1
fi

# The K3s token is read from the server over IAP and piped to each worker over
# its own IAP SSH session. It is never placed in Terraform state, metadata, or
# a command line. The token grants cluster-admin access; do not log it.
K3S_TOKEN="$(gcloud compute ssh "${APP}" --zone="${ZONE}" --tunnel-through-iap --quiet \
  --command='sudo cat /var/lib/rancher/k3s/server/node-token')"
[[ -n "${K3S_TOKEN}" ]] || { printf 'K3s token was empty.\n' >&2; exit 1; }

join_worker() {
  local instance="$1" node_name="$2" node_label="$3"
  printf 'Waiting for %s startup preparation...\n' "${instance}"
  local prepared=0
  for attempt in {1..60}; do
    if gcloud compute ssh "${instance}" --zone="${ZONE}" --tunnel-through-iap --quiet \
      --command='sudo test -x /usr/local/sbin/movierec-join-k3s-agent'; then
      prepared=1
      break
    fi
    sleep 10
  done
  if [[ "${prepared}" != 1 ]]; then
    printf '%s did not finish startup preparation within 10 minutes.\n' "${instance}" >&2
    exit 1
  fi
  printf 'Joining %s as %s...\n' "${instance}" "${node_name}"
  printf '%s\n' "${K3S_TOKEN}" | \
    gcloud compute ssh "${instance}" --zone="${ZONE}" --tunnel-through-iap --quiet \
      --command="sudo /usr/local/sbin/movierec-join-k3s-agent 'https://${APP_PRIVATE_IP}:6443' '${node_name}' '${node_label}'"
}

join_worker "${DB}" "${DB}" 'movierec.io/role=database'
join_worker "${MODEL}" "${MODEL}" 'movierec.io/role=model'
unset K3S_TOKEN

printf '\nCluster nodes:\n'
gcloud compute ssh "${APP}" --zone="${ZONE}" --tunnel-through-iap --quiet \
  --command="sudo k3s kubectl wait --for=condition=Ready node/${DB} node/${MODEL} --timeout=300s && sudo k3s kubectl get nodes --show-labels"
