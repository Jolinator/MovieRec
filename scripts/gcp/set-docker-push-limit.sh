#!/usr/bin/env bash
set -euo pipefail

MAX_CONCURRENT_UPLOADS="${1:-1}"
DAEMON_CONFIG=/etc/docker/daemon.json

if [[ "${EUID}" -ne 0 ]]; then
  printf 'Run this helper with sudo, for example: sudo bash %s 1\n' "$0" >&2
  exit 2
fi

if [[ ! "${MAX_CONCURRENT_UPLOADS}" =~ ^[1-9][0-9]*$ ]]; then
  printf 'Upload limit must be a positive integer; got: %s\n' "${MAX_CONCURRENT_UPLOADS}" >&2
  exit 2
fi

for command in python3 pgrep; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

python3 - "${DAEMON_CONFIG}" "${MAX_CONCURRENT_UPLOADS}" <<'PY'
import json
import os
import stat
import sys
import tempfile
from pathlib import Path

config_path = Path(sys.argv[1])
limit = int(sys.argv[2])
config_path.parent.mkdir(parents=True, exist_ok=True)

if config_path.exists():
    try:
        config = json.loads(config_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as error:
        raise SystemExit(f"Invalid JSON in {config_path}: {error}")
    if not isinstance(config, dict):
        raise SystemExit(f"Expected a JSON object in {config_path}")
    mode = stat.S_IMODE(config_path.stat().st_mode)
else:
    config = {}
    mode = 0o644

config["max-concurrent-uploads"] = limit
temporary_path = None
try:
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        dir=config_path.parent,
        prefix=f".{config_path.name}.",
        delete=False,
    ) as temporary_file:
        json.dump(config, temporary_file, indent=2)
        temporary_file.write("\n")
        temporary_path = Path(temporary_file.name)
    os.chmod(temporary_path, mode)
    os.replace(temporary_path, config_path)
finally:
    if temporary_path is not None and temporary_path.exists():
        temporary_path.unlink()
PY

if ! DOCKERD_PID="$(pgrep -xo dockerd)"; then
  printf 'Updated %s, but could not find the dockerd process to reload it.\n' "${DAEMON_CONFIG}" >&2
  exit 1
fi

if ! kill -HUP "${DOCKERD_PID}"; then
  printf 'Updated %s, but could not reload dockerd (PID %s).\n' "${DAEMON_CONFIG}" "${DOCKERD_PID}" >&2
  exit 1
fi

printf 'Set max-concurrent-uploads to %s and reloaded dockerd.\n' "${MAX_CONCURRENT_UPLOADS}"
