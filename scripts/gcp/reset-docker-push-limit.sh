#!/usr/bin/env bash
set -euo pipefail

DAEMON_CONFIG=/etc/docker/daemon.json

if [[ "${EUID}" -ne 0 ]]; then
  printf 'Run this helper with sudo, for example: sudo bash %s\n' "$0" >&2
  exit 2
fi

for command in python3 pgrep; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

if ! RESULT="$(python3 - "${DAEMON_CONFIG}" <<'PY'
import json
import os
import stat
import sys
import tempfile
from pathlib import Path

config_path = Path(sys.argv[1])
if not config_path.exists():
    print("absent")
    raise SystemExit(0)

try:
    config = json.loads(config_path.read_text(encoding="utf-8"))
except json.JSONDecodeError as error:
    raise SystemExit(f"Invalid JSON in {config_path}: {error}")
if not isinstance(config, dict):
    raise SystemExit(f"Expected a JSON object in {config_path}")

if "max-concurrent-uploads" not in config:
    print("unchanged")
    raise SystemExit(0)

del config["max-concurrent-uploads"]
if not config:
    config_path.unlink()
else:
    mode = stat.S_IMODE(config_path.stat().st_mode)
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

print("removed")
PY
)"; then
  exit 1
fi

if [[ "${RESULT}" == absent ]]; then
  printf '%s does not exist; Docker is already using its default upload limit.\n' "${DAEMON_CONFIG}"
  exit 0
fi

if [[ "${RESULT}" == unchanged ]]; then
  printf 'max-concurrent-uploads is not set in %s; no change needed.\n' "${DAEMON_CONFIG}"
  exit 0
fi

if ! DOCKERD_PID="$(pgrep -xo dockerd)"; then
  printf 'Removed the setting from %s, but could not find dockerd to reload it.\n' "${DAEMON_CONFIG}" >&2
  exit 1
fi

if ! kill -HUP "${DOCKERD_PID}"; then
  printf 'Removed the setting from %s, but could not reload dockerd (PID %s).\n' "${DAEMON_CONFIG}" "${DOCKERD_PID}" >&2
  exit 1
fi

printf 'Removed max-concurrent-uploads and reloaded dockerd; Docker will use its default.\n'
