#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if (($# != 1)); then
  printf 'Usage: %s POSITIVE_INTEGER\n' "${0##*/}" >&2
  exit 2
fi
exec python3 "${SCRIPT_DIR}/docker-push-limit.py" set "$1"
