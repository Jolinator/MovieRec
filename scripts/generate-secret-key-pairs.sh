#!/usr/bin/env bash
set -euo pipefail
umask 077

if [[ "$#" -ne 1 ]]; then
  printf 'Usage: %s SECRETS_DIR\n' "$0" >&2
  exit 2
fi

SECRETS_DIR="$1"
TLS_DIR="${SECRETS_DIR}/tls"

if ! command -v openssl >/dev/null 2>&1; then
  printf 'Required command not found: openssl\n' >&2
  exit 1
fi

mkdir -p "${TLS_DIR}"
chmod 700 "${SECRETS_DIR}" "${TLS_DIR}"

generate_tls_pair() {
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -keyout "${TLS_DIR}/server.key" \
    -out "${TLS_DIR}/server.crt" \
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
  local generator="$4"

  if [[ -e "${private_key}" && ! -s "${public_key}" ]] || \
     [[ -e "${public_key}" && ! -s "${private_key}" ]]; then
    printf '%s are incomplete; restore the pair or remove both before retrying.\n' "${label}" >&2
    exit 2
  fi

  if [[ ! -s "${private_key}" && ! -s "${public_key}" ]]; then
    "${generator}"
    printf 'Generated %s.\n' "${label}"
  fi

  chmod 600 "${private_key}" "${public_key}"
}

ensure_key_pair \
  'PostgreSQL TLS files' \
  "${TLS_DIR}/server.key" "${TLS_DIR}/server.crt" \
  generate_tls_pair

ensure_key_pair \
  'JWT key files' \
  "${SECRETS_DIR}/jwt-private.pem" "${SECRETS_DIR}/jwt-public.pem" \
  generate_jwt_pair
