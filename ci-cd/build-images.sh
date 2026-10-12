#!/usr/bin/env sh
set -eu

: "${BACKEND_IMAGE:?BACKEND_IMAGE must be set}"
: "${FRONTEND_IMAGE:?FRONTEND_IMAGE must be set}"

docker build \
  --file backend/Dockerfile \
  --tag "$BACKEND_IMAGE" \
  .

docker build \
  --file frontend/Dockerfile \
  --tag "$FRONTEND_IMAGE" \
  .
