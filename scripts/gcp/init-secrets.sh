#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
SECRETS_DIR="${REPO_ROOT}/deploy/gcp/secrets"
LOCAL_SECRETS_DIR="${REPO_ROOT}/secrets"
EXAMPLES_DIR="${REPO_ROOT}/deploy/gcp/secrets-examples"
SERVER_ENV="${SECRETS_DIR}/server.env"
LEGACY_SERVER_ENV="${SECRETS_DIR}/.env"
SERVER_ENV_EXAMPLE="${EXAMPLES_DIR}/server.env.example"

if ! command -v openssl >/dev/null 2>&1; then
  printf 'Required command not found: openssl\n' >&2
  exit 1
fi

if ! command -v gcloud >/dev/null 2>&1; then
  printf 'Required command not found: gcloud (needed to fill TF_VAR_project_id in server.env).\n' >&2
  exit 1
fi

PROJECT_ID="$(gcloud config get-value project 2>/dev/null || true)"
if [[ -z "${PROJECT_ID}" || "${PROJECT_ID}" == "(unset)" ]]; then
  printf 'No active Google Cloud project is configured. Select one with: gcloud config set project PROJECT_ID\n' >&2
  exit 1
fi

mkdir -p "${SECRETS_DIR}/tls"
chmod 700 "${SECRETS_DIR}" "${SECRETS_DIR}/tls"

if [[ ! -e "${SERVER_ENV}" ]]; then
  if [[ -e "${LEGACY_SERVER_ENV}" ]]; then
    mv "${LEGACY_SERVER_ENV}" "${SERVER_ENV}"
    printf 'Renamed the existing server settings file to %s.\n' "${SERVER_ENV}"
  else
    cp "${SERVER_ENV_EXAMPLE}" "${SERVER_ENV}"
    printf 'Created %s from its example.\n' "${SERVER_ENV}"
  fi
fi

if grep -Eq '^[[:space:]]*TF_VAR_project_id[[:space:]]*=' "${SERVER_ENV}"; then
  sed -i -E "s|^[[:space:]]*TF_VAR_project_id[[:space:]]*=.*|TF_VAR_project_id=${PROJECT_ID}|" "${SERVER_ENV}"
else
  printf 'TF_VAR_project_id=%s\n' "${PROJECT_ID}" >> "${SERVER_ENV}"
fi
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

if [[ ! -e "${SECRETS_DIR}/tls/server.key" && ! -e "${SECRETS_DIR}/tls/server.crt" ]] && \
   [[ -s "${LOCAL_SECRETS_DIR}/tls/server.key" && -s "${LOCAL_SECRETS_DIR}/tls/server.crt" ]]; then
  cp "${LOCAL_SECRETS_DIR}/tls/server.key" "${SECRETS_DIR}/tls/server.key"
  cp "${LOCAL_SECRETS_DIR}/tls/server.crt" "${SECRETS_DIR}/tls/server.crt"
  printf 'Copied configured local PostgreSQL TLS files.\n'
fi

if [[ -e "${SECRETS_DIR}/tls/server.key" && ! -s "${SECRETS_DIR}/tls/server.crt" ]] || \
   [[ -e "${SECRETS_DIR}/tls/server.crt" && ! -s "${SECRETS_DIR}/tls/server.key" ]]; then
  printf 'PostgreSQL TLS files are incomplete; restore the pair or remove both before retrying.\n' >&2
  exit 2
fi

if [[ ! -s "${SECRETS_DIR}/tls/server.key" && ! -s "${SECRETS_DIR}/tls/server.crt" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -keyout "${SECRETS_DIR}/tls/server.key" \
    -out "${SECRETS_DIR}/tls/server.crt" \
    -subj "/CN=postgres"
  printf 'Generated PostgreSQL TLS files.\n'
fi
chmod 600 "${SECRETS_DIR}/tls/server.key" "${SECRETS_DIR}/tls/server.crt"

if [[ ! -e "${SECRETS_DIR}/jwt-private.pem" && ! -e "${SECRETS_DIR}/jwt-public.pem" ]] && \
   [[ -s "${LOCAL_SECRETS_DIR}/jwt-private.pem" && -s "${LOCAL_SECRETS_DIR}/jwt-public.pem" ]]; then
  cp "${LOCAL_SECRETS_DIR}/jwt-private.pem" "${SECRETS_DIR}/jwt-private.pem"
  cp "${LOCAL_SECRETS_DIR}/jwt-public.pem" "${SECRETS_DIR}/jwt-public.pem"
  printf 'Copied configured local JWT keys.\n'
fi

if [[ -e "${SECRETS_DIR}/jwt-private.pem" && ! -s "${SECRETS_DIR}/jwt-public.pem" ]] || \
   [[ -e "${SECRETS_DIR}/jwt-public.pem" && ! -s "${SECRETS_DIR}/jwt-private.pem" ]]; then
  printf 'JWT key files are incomplete; restore the pair or remove both before retrying.\n' >&2
  exit 2
fi

if [[ ! -s "${SECRETS_DIR}/jwt-private.pem" && ! -s "${SECRETS_DIR}/jwt-public.pem" ]]; then
  openssl genrsa -out "${SECRETS_DIR}/jwt-private.pem" 2048
  openssl rsa -in "${SECRETS_DIR}/jwt-private.pem" \
    -pubout -out "${SECRETS_DIR}/jwt-public.pem"
  printf 'Generated JWT signing keys.\n'
fi
chmod 600 "${SECRETS_DIR}/jwt-private.pem" "${SECRETS_DIR}/jwt-public.pem"

printf '\nReview %s/*.env and set the TMDB token before running make k8s-config.\n' "${SECRETS_DIR}"
