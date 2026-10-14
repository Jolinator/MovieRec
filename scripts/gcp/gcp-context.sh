#!/usr/bin/env bash
set -euo pipefail

active_project_id() {
  if ! command -v gcloud >/dev/null 2>&1; then
    printf 'Required command not found: gcloud\n' >&2
    return 1
  fi

  local project_id
  project_id="$(gcloud config get-value project 2>/dev/null || true)"
  if [[ -z "${project_id}" || "${project_id}" == "(unset)" ]]; then
    printf 'No active Google Cloud project is configured. Select one with: gcloud config set project PROJECT_ID\n' >&2
    return 1
  fi

  printf '%s\n' "${project_id}"
}

resolve_project_id() {
  local project_id="${GCP_PROJECT_ID:-${TF_VAR_project_id:-}}"
  if [[ -z "${project_id}" ]]; then
    project_id="$(active_project_id)" || return 1
  fi

  if [[ -z "${project_id}" || "${project_id}" == "(unset)" ]]; then
    printf 'Set GCP_PROJECT_ID, TF_VAR_project_id, or select an active Google Cloud project.\n' >&2
    return 1
  fi

  printf '%s\n' "${project_id}"
}

case "${1:-}" in
  active-project-id) active_project_id ;;
  project-id) resolve_project_id ;;
  *)
    printf 'Usage: %s {active-project-id|project-id}\n' "$0" >&2
    exit 2
    ;;
esac
