#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
IMAGE_TAG="${1:-}"
PUBLIC_HOST_OVERRIDE="${2:-}"
MODEL_GPU_ENABLED="${MODEL_GPU_ENABLED:-false}"
GPU_LLAMA_IMAGE="${GPU_LLAMA_IMAGE:-ghcr.io/ggml-org/llama.cpp:server-cuda-v0.4.1}"

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
  printf 'Create it with make connect and keep the app IAP tunnel open.\n' >&2
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
if [[ "${MODEL_GPU_ENABLED}" != "true" && "${MODEL_GPU_ENABLED}" != "false" ]]; then
  printf 'MODEL_GPU_ENABLED must be true or false: %s\n' "${MODEL_GPU_ENABLED}" >&2
  exit 2
fi
TERRAFORM_GPU_ENABLED="$(terraform -chdir="${TF_DIR}" output -raw model_gpu_enabled 2>/dev/null || true)"
if [[ "${MODEL_GPU_ENABLED}" == "true" && "${TERRAFORM_GPU_ENABLED}" != "true" ]]; then
  printf 'GPU deployment requested, but Terraform model_gpu_enabled is not true. Set it and a GPU-compatible model_machine_type, then run make infra-apply.\n' >&2
  exit 2
fi
if [[ ! "${GPU_LLAMA_IMAGE}" =~ ^[A-Za-z0-9./_-]+:[A-Za-z0-9_.-]+$ ]]; then
  printf 'Invalid GPU llama image reference: %s\n' "${GPU_LLAMA_IMAGE}" >&2
  exit 2
fi
export REGISTRY IMAGE_TAG PUBLIC_HOST GPU_LLAMA_IMAGE

printf 'Kubernetes context: '
kubectl config current-context
if ! kubectl --request-timeout=10s get nodes; then
  printf 'Cannot reach the Kubernetes cluster. Check KUBECONFIG and keep the IAP tunnel open.\n' >&2
  exit 1
fi

printf 'Applying MovieRec manifests for image tag %s at %s\n' "${IMAGE_TAG}" "${PUBLIC_HOST}"
envsubst '${REGISTRY} ${IMAGE_TAG} ${PUBLIC_HOST}' \
  < "${REPO_ROOT}/deploy/k8s/movierec.yaml" | kubectl apply -f -

if [[ "${MODEL_GPU_ENABLED}" == "true" ]]; then
  printf 'Applying the opt-in CUDA generation overlay. Run make gpu-setup first.\n'
  envsubst '${GPU_LLAMA_IMAGE}' < "${REPO_ROOT}/deploy/k8s/movierec-gpu.yaml" | kubectl apply -f -
fi

# The API starts even when databases or llama.cpp are unavailable. Rollout status
# confirms the API process and frontend are serving; inspect /movieAPI/health for
# dependency readiness.
kubectl -n movierec rollout status deployment/backend --timeout=15m
kubectl -n movierec rollout status deployment/frontend --timeout=10m

printf 'Deployment applied. Check health at: https://%s/movieAPI/health\n' "${PUBLIC_HOST}"
