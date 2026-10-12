#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
SECRETS_DIR="${REPO_ROOT}/deploy/gcp/secrets"
LOCAL_SECRETS_DIR="${REPO_ROOT}/secrets"
EXAMPLES_DIR="${REPO_ROOT}/deploy/gcp/secrets-examples"

if ! command -v openssl >/dev/null 2>&1; then
  printf 'Required command not found: openssl\n' >&2
  exit 1
fi

mkdir -p "${SECRETS_DIR}/tls"
chmod 700 "${SECRETS_DIR}" "${SECRETS_DIR}/tls"

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
