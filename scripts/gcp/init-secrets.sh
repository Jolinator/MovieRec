#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
SECRETS_DIR="${REPO_ROOT}/deploy/gcp/secrets"
LOCAL_SECRETS_DIR="${REPO_ROOT}/secrets"
EXAMPLES_DIR="${REPO_ROOT}/deploy/gcp/secrets-examples"
SERVER_ENV="${SECRETS_DIR}/server.env"
SERVER_ENV_EXAMPLE="${EXAMPLES_DIR}/server.env.example"
GCP_CONFIG="${SCRIPT_DIR}/gcp-config.py"

if ! command -v openssl >/dev/null 2>&1; then
  printf 'Required command not found: openssl\n' >&2
  exit 1
fi

PROJECT_ID="$(bash "${SCRIPT_DIR}/gcp-context.sh" active-project-id)"

mkdir -p "${SECRETS_DIR}/tls"
chmod 700 "${SECRETS_DIR}" "${SECRETS_DIR}/tls"

if [[ ! -e "${SERVER_ENV}" ]]; then
  cp "${SERVER_ENV_EXAMPLE}" "${SERVER_ENV}"
  printf 'Created %s from its example.\n' "${SERVER_ENV}"
fi

python3 "${GCP_CONFIG}" set-server-env TF_VAR_project_id "${PROJECT_ID}"
chmod 600 "${SERVER_ENV}"
printf 'Set TF_VAR_project_id to the active gcloud project in %s.\n' "${SERVER_ENV}"

for service in auth movies recommendations tmdb; do
  target="${SECRETS_DIR}/${service}.env"
  example="${EXAMPLES_DIR}/${service}.env.example"
  if [[ ! -e "${target}" ]]; then
    if [[ -f "${LOCAL_SECRETS_DIR}/${service}.env" ]]; then
      cp "${LOCAL_SECRETS_DIR}/${service}.env" "${target}"
      printf 'Copied configured local values into %s.\n' "${target}"
    else
      cp "${example}" "${target}"
      printf 'Created %s from its example; replace its placeholder values before deployment.\n' "${target}"
    fi
  else
    printf 'Keeping existing %s\n' "${target}"
  fi
  chmod 600 "${target}"
done

generate_tls_pair() {
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -keyout "${SECRETS_DIR}/tls/server.key" \
    -out "${SECRETS_DIR}/tls/server.crt" \
    -subj "/CN=postgres"
}

generate_jwt_pair() {
  openssl genrsa -out "${SECRETS_DIR}/jwt-private.pem" 2048
  openssl rsa -in "${SECRETS_DIR}/jwt-private.pem" \
    -pubout -out "${SECRETS_DIR}/jwt-public.pem"
}

ensure_key_pair() {
  local label="$1"
  local local_private="$2"
  local local_public="$3"
  local private_key="$4"
  local public_key="$5"
  local copy_message="$6"
  local generated_message="$7"
  local generator="$8"

  if [[ ! -e "${private_key}" && ! -e "${public_key}" ]] && \
     [[ -s "${local_private}" && -s "${local_public}" ]]; then
    cp "${local_private}" "${private_key}"
    cp "${local_public}" "${public_key}"
    printf '%s\n' "${copy_message}"
  fi

  if [[ -e "${private_key}" && ! -s "${public_key}" ]] || \
     [[ -e "${public_key}" && ! -s "${private_key}" ]]; then
    printf '%s are incomplete; restore the pair or remove both before retrying.\n' "${label}" >&2
    exit 2
  fi

  if [[ ! -s "${private_key}" && ! -s "${public_key}" ]]; then
    "${generator}"
    printf '%s\n' "${generated_message}"
  fi

  chmod 600 "${private_key}" "${public_key}"
}

ensure_key_pair \
  'PostgreSQL TLS files' \
  "${LOCAL_SECRETS_DIR}/tls/server.key" "${LOCAL_SECRETS_DIR}/tls/server.crt" \
  "${SECRETS_DIR}/tls/server.key" "${SECRETS_DIR}/tls/server.crt" \
  'Copied configured local PostgreSQL TLS files.' \
  'Generated PostgreSQL TLS files.' \
  generate_tls_pair

ensure_key_pair \
  'JWT key files' \
  "${LOCAL_SECRETS_DIR}/jwt-private.pem" "${LOCAL_SECRETS_DIR}/jwt-public.pem" \
  "${SECRETS_DIR}/jwt-private.pem" "${SECRETS_DIR}/jwt-public.pem" \
  'Copied configured local JWT keys.' \
  'Generated JWT signing keys.' \
  generate_jwt_pair

printf '\nReview %s/*.env and set the TMDB token before running make k8s-config.\n' "${SECRETS_DIR}"
