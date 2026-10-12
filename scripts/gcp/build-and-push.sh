#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"
IMAGE_TAG="${1:-$(date -u +%Y%m%d%H%M%S)}"

if [[ ! "${IMAGE_TAG}" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
  printf 'Invalid Docker image tag: %s\n' "${IMAGE_TAG}" >&2
  exit 2
fi

for command in docker gcloud terraform; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

REGION="$(terraform -chdir="${TF_DIR}" output -raw region)"
REGISTRY="$(terraform -chdir="${TF_DIR}" output -raw artifact_registry)"

if [[ -z "${REGION}" || -z "${REGISTRY}" ]]; then
  printf 'Terraform outputs for region and artifact_registry must be available.\n' >&2
  exit 1
fi

printf 'Building and pushing MovieRec images with tag %s\n' "${IMAGE_TAG}"
printf 'Registry: %s\n' "${REGISTRY}"

gcloud auth configure-docker "${REGION}-docker.pkg.dev"

docker build -f "${REPO_ROOT}/backend/Dockerfile" \
  -t "${REGISTRY}/backend:${IMAGE_TAG}" "${REPO_ROOT}"
docker build -f "${REPO_ROOT}/frontend/Dockerfile" \
  --build-arg VITE_API_BASE_URL=/ \
  -t "${REGISTRY}/frontend:${IMAGE_TAG}" "${REPO_ROOT}"

docker push "${REGISTRY}/backend:${IMAGE_TAG}"
docker push "${REGISTRY}/frontend:${IMAGE_TAG}"

printf '\nUse this tag in the Kubernetes deploy step:\nIMAGE_TAG=%s\n' "${IMAGE_TAG}"
