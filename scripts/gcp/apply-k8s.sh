#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
IMAGE_TAG="${1:-}"
PUBLIC_HOST_OVERRIDE="${2:-}"

if [[ -z "${IMAGE_TAG}" ]]; then
  printf 'Usage: bash scripts/gcp/apply-k8s.sh IMAGE_TAG [PUBLIC_HOST]\n' >&2
  exit 2
fi

if [[ ! "${IMAGE_TAG}" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
  printf 'Invalid Docker image tag: %s\n' "${IMAGE_TAG}" >&2
  exit 2
fi

for command in envsubst kubectl terraform; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    if [[ "${command}" == envsubst ]]; then
      printf 'Install it with: sudo apt-get update && sudo apt-get install -y gettext-base\n' >&2
    fi
    exit 1
  fi
done

export KUBECONFIG="${KUBECONFIG:-${REPO_ROOT}/kubeconfig}"
if [[ ! -r "${KUBECONFIG}" ]]; then
  printf 'Kubeconfig is not readable: %s\n' "${KUBECONFIG}" >&2
  printf 'Create it using the instructions in TODO.md section 6.\n' >&2
  exit 1
fi

REGISTRY="$(terraform -chdir="${TF_DIR}" output -raw artifact_registry)"
PUBLIC_IP="$(terraform -chdir="${TF_DIR}" output -raw public_ip)"
if [[ -z "${REGISTRY}" || -z "${PUBLIC_IP}" ]]; then
  printf 'Terraform outputs artifact_registry and public_ip must be available.\n' >&2
  exit 1
fi

PUBLIC_HOST="${PUBLIC_HOST_OVERRIDE:-${PUBLIC_HOST:-${PUBLIC_IP}.nip.io}}"
if [[ ! "${PUBLIC_HOST}" =~ ^[A-Za-z0-9.-]+$ ]]; then
  printf 'Invalid public host; use a DNS name or IPv4 address: %s\n' "${PUBLIC_HOST}" >&2
  exit 2
fi
export REGISTRY IMAGE_TAG PUBLIC_HOST

printf 'Kubernetes context: '
kubectl config current-context
if ! kubectl --request-timeout=10s get nodes; then
  printf 'Cannot reach the Kubernetes cluster. Check KUBECONFIG and keep the IAP tunnel open.\n' >&2
  exit 1
fi

printf 'Applying MovieRec manifests for image tag %s at %s\n' "${IMAGE_TAG}" "${PUBLIC_HOST}"
envsubst '${REGISTRY} ${IMAGE_TAG} ${PUBLIC_HOST}' \
  < "${REPO_ROOT}/deploy/k8s/movierec.yaml" | kubectl apply -f -

# The API starts even when databases or llama.cpp are unavailable. Rollout status
# confirms the API process and frontend are serving; inspect /movieAPI/health for
# dependency readiness.
kubectl -n movierec rollout status deployment/backend --timeout=15m
kubectl -n movierec rollout status deployment/frontend --timeout=10m

printf 'Deployment applied. Check health at: https://%s/movieAPI/health\n' "${PUBLIC_HOST}"
