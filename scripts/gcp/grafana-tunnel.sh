#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${HOME:?HOME must be set}/.cache/movierec/grafana-tunnels"

tunnel_group_has_grafana() {
  local group_id="$1"
  ps -eo pgid=,args= | awk -v target="${group_id}" \
    '$1 == target && /port-forward service\/grafana/ { found = 1 } END { exit !found }'
}

stop_tunnels() {
  local pid_file group_id attempt
  local -a pid_files=()

  shopt -s nullglob
  pid_files=("${STATE_DIR}"/*.pid)
  if ((${#pid_files[@]} == 0)); then
    printf 'No tracked Grafana tunnels for this local user.\n'
    return 0
  fi

  for pid_file in "${pid_files[@]}"; do
    group_id="${pid_file##*/}"
    group_id="${group_id%.pid}"
    if [[ ! "${group_id}" =~ ^[0-9]+$ ]]; then
      rm -f -- "${pid_file}"
      continue
    fi

    if tunnel_group_has_grafana "${group_id}"; then
      kill -TERM -- "-${group_id}" 2>/dev/null || true
      for ((attempt = 0; attempt < 5; attempt++)); do
        tunnel_group_has_grafana "${group_id}" || break
        sleep 1
      done
      if tunnel_group_has_grafana "${group_id}"; then
        kill -KILL -- "-${group_id}" 2>/dev/null || true
      fi
      printf 'Stopped Grafana tunnel process group %s.\n' "${group_id}"
    else
      printf 'Removed stale Grafana tunnel record %s.\n' "${group_id}"
    fi
    rm -f -- "${pid_file}"
  done
}

start_tunnel() {
  local grafana_local_port grafana_remote_port grafana_local_bind remote_command
  local tunnel_pid pid_file

  command -v setsid >/dev/null 2>&1 || {
    printf 'Required command not found: setsid\n' >&2
    return 2
  }

  umask 077
  mkdir -p -- "${STATE_DIR}"
  chmod 700 "${STATE_DIR}"

  grafana_local_port=$((20000 + RANDOM % 20000))
  grafana_remote_port=$((40000 + RANDOM % 20000))
  grafana_local_bind="127.0.0.1"
  if [[ -n "${WEB_HOST:-}" ]]; then
    [[ "${WEB_HOST}" =~ ^[A-Za-z0-9.-]+$ ]] || {
      printf '%s\n' 'WEB_HOST contains invalid characters.' >&2
      return 2
    }
    grafana_local_bind="0.0.0.0"
    printf 'Grafana: https://%s-%s/\n' "${grafana_local_port}" "${WEB_HOST}"
  else
    printf 'Open http://localhost:%s in a local browser.\n' "${grafana_local_port}"
  fi

  printf '%s\n' \
    'Configuring stable Grafana proxy settings; this only rolls out Grafana if they changed.' \
    'Press Ctrl-C to close the tunnel, or run make grafana-stop to close all your Grafana tunnels.'

  remote_command="sudo k3s kubectl -n movierec set env deployment/grafana GF_SERVER_ROOT_URL=http://localhost:3000/ GF_SECURITY_CSRF_ADDITIONAL_HEADERS=X-Forwarded-Host 'GF_LIVE_ALLOWED_ORIGINS=https://*-cs-*.cloudshell.dev,http://localhost:*,http://127.0.0.1:*' GF_SECURITY_CSRF_TRUSTED_ORIGINS- && sudo k3s kubectl -n movierec rollout status deployment/grafana --timeout=5m && sudo k3s kubectl -n movierec port-forward service/grafana ${grafana_remote_port}:3000 --address 127.0.0.1"

  setsid bash "${SCRIPT_DIR}/ssh-machine.sh" app \
    --ssh-flag="-L ${grafana_local_bind}:${grafana_local_port}:localhost:${grafana_remote_port}" \
    --command="${remote_command}" &
  tunnel_pid=$!
  pid_file="${STATE_DIR}/${tunnel_pid}.pid"
  printf '%s\n' "${tunnel_pid}" > "${pid_file}"

  cleanup() {
    if tunnel_group_has_grafana "${tunnel_pid}"; then
      kill -TERM -- "-${tunnel_pid}" 2>/dev/null || true
    fi
    wait "${tunnel_pid}" 2>/dev/null || true
    rm -f -- "${pid_file}"
  }
  trap cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  wait "${tunnel_pid}"
}

case "${1:-}" in
  start) start_tunnel ;;
  stop) stop_tunnels ;;
  *)
    printf 'Usage: %s {start|stop}\n' "${0##*/}" >&2
    exit 2
    ;;
esac
