#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  printf 'Usage: %s {app|database|model} [--wait-command COMMAND] [--timeout-seconds SECONDS]\n' "${0##*/}"
  printf 'Reboots the VM through SSH, then waits for SSH and COMMAND to work again.\n'
}

if (($# == 0)); then
  usage >&2
  exit 2
fi

machine="$1"
shift
case "${machine}" in
  app|database|model) ;;
  *)
    printf 'Unknown machine role: %s\n' "${machine}" >&2
    usage >&2
    exit 2
    ;;
esac

wait_command=true
timeout_seconds=600
while (($#)); do
  case "$1" in
    --wait-command)
      (($# >= 2)) || { usage >&2; exit 2; }
      wait_command="$2"
      shift 2
      ;;
    --timeout-seconds)
      (($# >= 2)) || { usage >&2; exit 2; }
      timeout_seconds="$2"
      shift 2
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${wait_command}" ]]; then
  printf 'Wait command cannot be empty.\n' >&2
  exit 2
fi
if [[ ! "${timeout_seconds}" =~ ^[1-9][0-9]*$ ]]; then
  printf 'Timeout must be a positive number of seconds.\n' >&2
  exit 2
fi

ssh_machine() {
  bash "${SCRIPT_DIR}/ssh-machine.sh" "${machine}" --command "$1"
}

if ! ssh_machine true >/dev/null; then
  printf 'Cannot reach %s over SSH; no reboot was requested.\n' "${machine}" >&2
  exit 1
fi

printf 'Requesting guest OS reboot for %s.\n' "${machine}"
# SSH may disconnect as soon as the guest starts shutting down.
ssh_machine 'sudo reboot' >/dev/null 2>&1 || true

printf 'Waiting for %s to reboot and become reachable again.\n' "${machine}"
disconnected=false
elapsed=0
while ((elapsed < timeout_seconds)); do
  if ssh_machine "${wait_command}" >/dev/null 2>&1; then
    if [[ "${disconnected}" == true ]]; then
      printf '%s rebooted and is ready.\n' "${machine}"
      exit 0
    fi
  else
    disconnected=true
  fi
  sleep 5
  elapsed=$((elapsed + 5))
done

if [[ "${disconnected}" != true ]]; then
  printf 'Timed out waiting for %s to disconnect and reboot.\n' "${machine}" >&2
else
  printf 'Timed out waiting for %s to return and pass its readiness command.\n' "${machine}" >&2
fi
exit 1
