#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
IMAGE_TAG="${1:-$(date -u +%Y%m%d%H%M%S)}"
PROJECT_ID="${GCP_PROJECT_ID:-${TF_VAR_project_id:-}}"
REGION="${GCP_REGION:-europe-west1}"
RESOURCE_NAME="${MOVIEREC_RESOURCE_NAME:-movierec}"

if [[ ! "${IMAGE_TAG}" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
  printf 'Invalid Docker image tag: %s\n' "${IMAGE_TAG}" >&2
  exit 2
fi

for command in docker gcloud; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

if [[ -z "${PROJECT_ID}" ]]; then
  PROJECT_ID="$(gcloud config get-value project 2>/dev/null || true)"
fi

if [[ -z "${PROJECT_ID}" || "${PROJECT_ID}" == "(unset)" ]]; then
  printf 'Set GCP_PROJECT_ID, TF_VAR_project_id, or an active gcloud project.\n' >&2
  exit 1
fi
if [[ -z "${REGION}" || -z "${RESOURCE_NAME}" ]]; then
  printf 'GCP_REGION and MOVIEREC_RESOURCE_NAME must not be empty.\n' >&2
  exit 1
fi

REGISTRY="${REGION}-docker.pkg.dev/${PROJECT_ID}/${RESOURCE_NAME}"

printf 'Building and pushing MovieRec images with tag %s\n' "${IMAGE_TAG}"
printf 'Registry: %s\n' "${REGISTRY}"

gcloud auth configure-docker "${REGION}-docker.pkg.dev"

for service in gateway auth movies recommendations; do
  docker build -f "${REPO_ROOT}/backend/Dockerfile" \
    --build-arg "SERVICE=${service}" \
    -t "${REGISTRY}/${service}:${IMAGE_TAG}" "${REPO_ROOT}"
done
docker build -f "${REPO_ROOT}/frontend/Dockerfile" \
  --build-arg VITE_API_BASE_URL=/ \
  -t "${REGISTRY}/frontend:${IMAGE_TAG}" "${REPO_ROOT}"

for service in gateway auth movies recommendations frontend; do
  docker push "${REGISTRY}/${service}:${IMAGE_TAG}"
done

printf '\nUse this tag in the Kubernetes deploy step:\nIMAGE_TAG=%s\n' "${IMAGE_TAG}"
