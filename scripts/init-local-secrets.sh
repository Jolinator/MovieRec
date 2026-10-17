#!/usr/bin/env bash
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
SECRETS_DIR="${REPO_ROOT}/secrets"
EXAMPLES_DIR="${REPO_ROOT}/secrets-example"

for command in openssl python3; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

mkdir -p "${SECRETS_DIR}/tls"
chmod 700 "${SECRETS_DIR}" "${SECRETS_DIR}/tls"

for name in auth movies recommendations tmdb ports; do
  target="${SECRETS_DIR}/${name}.env"
  example="${EXAMPLES_DIR}/${name}.env.example"
  if [[ ! -e "${target}" ]]; then
    cp "${example}" "${target}"
    printf 'Created %s\n' "${target}"
  fi
  chmod 600 "${target}"
done

python3 "${REPO_ROOT}/scripts/generate-db-passwords.py" --secrets-dir "${SECRETS_DIR}"
bash "${REPO_ROOT}/scripts/generate-secret-key-pairs.sh" "${SECRETS_DIR}"

printf 'Local secrets are ready in %s.\n' "${SECRETS_DIR}"
