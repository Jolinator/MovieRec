#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
SECRETS_DIR="${REPO_ROOT}/deploy/gcp/secrets"
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
    cp "${example}" "${target}"
    printf 'Created %s from its GCP example; replace any placeholders before deployment.\n' "${target}"
  else
    if [[ "${service}" == tmdb ]]; then
      printf 'Keeping existing %s; TMDB_IMPORT_LIMIT will be set to 5000.\n' "${target}"
    else
      printf 'Keeping existing %s\n' "${target}"
    fi
  fi
  chmod 600 "${target}"
done

tmdb_env="${SECRETS_DIR}/tmdb.env"
if grep -q '^TMDB_IMPORT_LIMIT=' "${tmdb_env}"; then
  sed -i 's/^TMDB_IMPORT_LIMIT=.*/TMDB_IMPORT_LIMIT=5000/' "${tmdb_env}"
else
  printf '\nTMDB_IMPORT_LIMIT=5000\n' >> "${tmdb_env}"
fi
chmod 600 "${tmdb_env}"
printf 'Set TMDB_IMPORT_LIMIT to 5000 in %s.\n' "${tmdb_env}"

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
  local private_key="$2"
  local public_key="$3"
  local generated_message="$4"
  local generator="$5"

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
  "${SECRETS_DIR}/tls/server.key" "${SECRETS_DIR}/tls/server.crt" \
  'Generated PostgreSQL TLS files.' \
  generate_tls_pair

ensure_key_pair \
  'JWT key files' \
  "${SECRETS_DIR}/jwt-private.pem" "${SECRETS_DIR}/jwt-public.pem" \
  'Generated JWT signing keys.' \
  generate_jwt_pair

printf '\nReview %s/*.env and set the TMDB token before running make k8s-config.\n' "${SECRETS_DIR}"
