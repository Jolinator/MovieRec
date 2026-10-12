#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
SECRETS_DIR="${REPO_ROOT}/deploy/gcp/secrets"
NAMESPACE="${NAMESPACE:-movierec}"
export KUBECONFIG="${KUBECONFIG:-${REPO_ROOT}/kubeconfig}"

for command in kubectl openssl terraform; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

if [[ ! -r "${KUBECONFIG}" ]]; then
  printf 'Kubeconfig is not readable: %s\n' "${KUBECONFIG}" >&2
  printf 'Run make connect in another terminal and export KUBECONFIG first.\n' >&2
  exit 1
fi

if ! kubectl --request-timeout=10s get nodes >/dev/null; then
  printf 'Cannot reach the Kubernetes cluster. Keep the IAP tunnel open.\n' >&2
  exit 1
fi

for role in app database model; do
  if ! kubectl get nodes -l "movierec.io/role=${role}" --no-headers 2>/dev/null | grep -q .; then
    printf 'No cluster node has role label movierec.io/role=%s. Run make cluster-bootstrap first.\n' "${role}" >&2
    exit 1
  fi
done

required_files=(
  auth.env
  movies.env
  recommendations.env
  tmdb.env
  tls/server.crt
  tls/server.key
  jwt-private.pem
  jwt-public.pem
)
for relative_path in "${required_files[@]}"; do
  if [[ ! -s "${SECRETS_DIR}/${relative_path}" ]]; then
    printf 'Missing cloud secret file: %s\n' "${SECRETS_DIR}/${relative_path}" >&2
    printf 'Run make secrets-init, review the files, then retry.\n' >&2
    exit 2
  fi
done

for service in auth movies recommendations tmdb; do
  file="${SECRETS_DIR}/${service}.env"
  if grep -q 'REPLACE_' "${file}"; then
    printf 'Replace placeholder values in %s before continuing.\n' "${file}" >&2
    exit 2
  fi
done

PUBLIC_IP="$(terraform -chdir="${TF_DIR}" output -raw public_ip)"
PUBLIC_HOST="${PUBLIC_HOST:-${PUBLIC_IP}.nip.io}"
if [[ ! "${PUBLIC_HOST}" =~ ^[A-Za-z0-9.-]+$ ]]; then
  printf 'Invalid PUBLIC_HOST; use a DNS name or IPv4 address: %s\n' "${PUBLIC_HOST}" >&2
  exit 2
fi

if [[ "${PUBLIC_HOST}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
  WEB_SAN="IP:${PUBLIC_HOST}"
  WEB_SAN_CHECK="IP Address:${PUBLIC_HOST}"
else
  WEB_SAN="DNS:${PUBLIC_HOST}"
  WEB_SAN_CHECK="DNS:${PUBLIC_HOST}"
fi

if [[ -e "${SECRETS_DIR}/tls/web.key" && ! -s "${SECRETS_DIR}/tls/web.crt" ]] || \
   [[ -e "${SECRETS_DIR}/tls/web.crt" && ! -s "${SECRETS_DIR}/tls/web.key" ]]; then
  printf 'HTTPS certificate files are incomplete; restore the pair or remove both before retrying.\n' >&2
  exit 2
fi

if [[ ! -s "${SECRETS_DIR}/tls/web.key" && ! -s "${SECRETS_DIR}/tls/web.crt" ]] || \
   ! openssl x509 -in "${SECRETS_DIR}/tls/web.crt" -noout -ext subjectAltName 2>/dev/null | grep -Fq "${WEB_SAN_CHECK}"; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -keyout "${SECRETS_DIR}/tls/web.key" \
    -out "${SECRETS_DIR}/tls/web.crt" \
    -subj "/CN=${PUBLIC_HOST}" \
    -addext "subjectAltName=${WEB_SAN}"
  chmod 600 "${SECRETS_DIR}/tls/web.key"
fi

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

for service in auth movies recommendations; do
  kubectl create configmap "${service}-migrations" --namespace "${NAMESPACE}" \
    --from-file="${REPO_ROOT}/database/migrations/${service}" \
    --dry-run=client -o yaml | kubectl apply -f -
done

kubectl create configmap postgres-tls-entrypoint --namespace "${NAMESPACE}" \
  --from-file=tls-entrypoint.sh="${REPO_ROOT}/scripts/postgres-tls-entrypoint.sh" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create configmap model-downloader-script --namespace "${NAMESPACE}" \
  --from-file=download-models.sh="${REPO_ROOT}/scripts/download-models.sh" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create configmap prometheus-config --namespace "${NAMESPACE}" \
  --from-file=prometheus.yml="${REPO_ROOT}/monitoring/prometheus/prometheus.yml" \
  --dry-run=client -o yaml | kubectl apply -f -

for service in auth movies recommendations tmdb; do
  kubectl create secret generic "${service}-config" --namespace "${NAMESPACE}" \
    --from-env-file="${SECRETS_DIR}/${service}.env" \
    --dry-run=client -o yaml | kubectl apply -f -
done

kubectl create secret generic postgres-tls --namespace "${NAMESPACE}" \
  --from-file=server.crt="${SECRETS_DIR}/tls/server.crt" \
  --from-file=server.key="${SECRETS_DIR}/tls/server.key" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic jwt-keys --namespace "${NAMESPACE}" \
  --from-file=jwt-private.pem="${SECRETS_DIR}/jwt-private.pem" \
  --from-file=jwt-public.pem="${SECRETS_DIR}/jwt-public.pem" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret tls movierec-web-tls --namespace "${NAMESPACE}" \
  --cert="${SECRETS_DIR}/tls/web.crt" --key="${SECRETS_DIR}/tls/web.key" \
  --dry-run=client -o yaml | kubectl apply -f -

if kubectl -n "${NAMESPACE}" get secret grafana-config >/dev/null 2>&1; then
  printf 'Keeping existing Grafana password secret.\n'
else
  read -r -s -p 'Grafana admin password: ' GRAFANA_PASSWORD
  printf '\n'
  if [[ -z "${GRAFANA_PASSWORD}" ]]; then
    printf 'Grafana password cannot be empty.\n' >&2
    exit 2
  fi
  printf '%s' "${GRAFANA_PASSWORD}" | kubectl create secret generic grafana-config \
    --namespace "${NAMESPACE}" --from-file=admin-password=/dev/stdin \
    --dry-run=client -o yaml | kubectl apply -f -
  unset GRAFANA_PASSWORD
fi

printf 'Kubernetes namespace, ConfigMaps, secrets, and TLS certificate are ready.\n'
printf 'Using HTTPS host: %s\n' "${PUBLIC_HOST}"
