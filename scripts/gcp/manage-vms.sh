#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-}"
if [[ "${ACTION}" != start && "${ACTION}" != stop ]]; then
  printf 'Usage: %s {start|stop}\n' "$0" >&2
  exit 2
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ID="$(bash "${SCRIPT_DIR}/gcp-context.sh" project-id)"
ZONE="${GCP_ZONE:-}"
RESOURCE_NAME="${MOVIEREC_RESOURCE_NAME:-}"

if [[ -z "${ZONE}" || -z "${RESOURCE_NAME}" ]]; then
  printf 'Set GCP_ZONE and MOVIEREC_RESOURCE_NAME before running make %s.\n' "${ACTION}" >&2
  exit 2
fi

STATUS=0
for ROLE in app database model; do
  INSTANCE="${RESOURCE_NAME}-${ROLE}"
  if ! INSTANCE_STATUS="$(gcloud compute instances describe "${INSTANCE}" --project="${PROJECT_ID}" --zone="${ZONE}" --format='value(status)' 2>/dev/null)"; then
    printf 'Could not read status for %s\n' "${INSTANCE}" >&2
    STATUS=1
    continue
  fi

  if [[ "${ACTION}" == start ]]; then
    if [[ "${INSTANCE_STATUS}" == RUNNING || "${INSTANCE_STATUS}" == PROVISIONING || "${INSTANCE_STATUS}" == STAGING ]]; then
      printf '%s already running (status: %s)\n' "${INSTANCE}" "${INSTANCE_STATUS}"
      continue
    fi
    if ! gcloud compute instances start "${INSTANCE}" --project="${PROJECT_ID}" --zone="${ZONE}"; then
      printf 'Failed to start %s\n' "${INSTANCE}" >&2
      STATUS=1
      continue
    fi
    if ! gcloud compute instances describe "${INSTANCE}" --project="${PROJECT_ID}" --zone="${ZONE}" --format='value(status)'; then
      printf 'Could not read status for %s\n' "${INSTANCE}" >&2
      STATUS=1
    fi
  else
    if [[ "${INSTANCE_STATUS}" == TERMINATED ]]; then
      printf '%s is already stopped\n' "${INSTANCE}"
      continue
    fi
    if ! gcloud compute instances stop "${INSTANCE}" --project="${PROJECT_ID}" --zone="${ZONE}"; then
      printf 'Failed to stop %s; check its state because it may still be billing\n' "${INSTANCE}" >&2
      STATUS=1
    fi
  fi
done

exit "${STATUS}"
