#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
SECRETS_DIR="${REPO_ROOT}/deploy/gcp/secrets"
EXAMPLES_DIR="${REPO_ROOT}/deploy/gcp/secrets-examples"
SERVER_ENV="${SECRETS_DIR}/server.env"

if ! command -v openssl >/dev/null 2>&1; then
  printf 'Required command not found: openssl\n' >&2
  exit 1
fi

if [[ ! -e "${SERVER_ENV}" ]]; then
  printf 'Missing %s; run make glean-vars first.\n' "${SERVER_ENV}" >&2
  exit 2
fi

mkdir -p "${SECRETS_DIR}/tls"
chmod 700 "${SECRETS_DIR}" "${SECRETS_DIR}/tls"
chmod 600 "${SERVER_ENV}"

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

bash "${REPO_ROOT}/scripts/generate-secret-key-pairs.sh" "${SECRETS_DIR}"

printf '\nReview %s/*.env and set the TMDB token before running make k8s-config.\n' "${SECRETS_DIR}"
