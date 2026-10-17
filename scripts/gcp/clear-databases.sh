#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

remote_command="$(cat <<'REMOTE'
set -euo pipefail
kubectl() { sudo k3s kubectl --namespace movierec "$@"; }
databases=(auth-postgres movies-postgres recommendations-postgres)

printf '%s\n' 'Checking that all PostgreSQL StatefulSets exist.'
for database in "${databases[@]}"; do
  kubectl get "statefulset/${database}" >/dev/null
done

printf '%s\n' 'Stopping PostgreSQL pods.'
for database in "${databases[@]}"; do
  kubectl scale "statefulset/${database}" --replicas=0
done

for database in "${databases[@]}"; do
  pod="${database}-0"
  existing_pod="$(kubectl get "pod/${pod}" --output=name --ignore-not-found)"
  if [[ -n "${existing_pod}" ]]; then
    kubectl wait --for=delete "${existing_pod}" --timeout=180s
  fi
done

printf '%s\n' 'Deleting the three PostgreSQL data claims.'
for database in "${databases[@]}"; do
  kubectl delete pvc "data-${database}-0" --ignore-not-found --wait=true --timeout=180s
done

printf '%s\n' 'Starting PostgreSQL with fresh data claims.'
for database in "${databases[@]}"; do
  kubectl scale "statefulset/${database}" --replicas=1
done
for database in "${databases[@]}"; do
  kubectl rollout status "statefulset/${database}" --timeout=300s
done

printf '%s\n' 'PostgreSQL databases are ready with empty data directories.'
REMOTE
)"

bash "${SCRIPT_DIR}/ssh-machine.sh" app --command "${remote_command}"
