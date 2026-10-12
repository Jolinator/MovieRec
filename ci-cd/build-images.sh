#!/usr/bin/env sh
set -eu

: "${BACKEND_IMAGE_PREFIX:?BACKEND_IMAGE_PREFIX must be set}"
: "${BACKEND_IMAGE_TAG:?BACKEND_IMAGE_TAG must be set}"
: "${FRONTEND_IMAGE:?FRONTEND_IMAGE must be set}"

# One image per backend service, all built from backend/Dockerfile.
for service in gateway auth movies recommendations; do
  docker build \
    --file backend/Dockerfile \
    --build-arg "SERVICE=$service" \
    --tag "$BACKEND_IMAGE_PREFIX/$service:$BACKEND_IMAGE_TAG" \
    .
done

docker build \
  --file frontend/Dockerfile \
  --tag "$FRONTEND_IMAGE" \
  .
