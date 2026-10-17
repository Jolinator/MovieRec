#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  printf 'Usage: %s [WORKLOAD_OR_RESOURCE] [--container NAME] [--namespace NAME] [--tail LINES] [--follow]\n' "${0##*/}"
  printf 'When WORKLOAD is omitted, --container discovers a matching workload in the namespace.\n'
  printf 'A workload name without a resource kind is treated as a Deployment.\n'
  printf 'Examples: %s llama-generation --container llama-server --follow\n' "${0##*/}"
  printf '          %s --container recommendations --follow\n' "${0##*/}"
  printf '          %s statefulset/movies-postgres --namespace movierec\n' "${0##*/}"
}

resource=""
resource_inferred="false"
if (($#)) && [[ "$1" != -* ]]; then
  resource="$1"
  shift
fi

namespace="movierec"
container=""
tail_lines="100"
follow="false"

while (($#)); do
  case "$1" in
    --container)
      (($# >= 2)) || { usage >&2; exit 2; }
      container="$2"
      shift 2
      ;;
    --namespace)
      (($# >= 2)) || { usage >&2; exit 2; }
      namespace="$2"
      shift 2
      ;;
    --tail)
      (($# >= 2)) || { usage >&2; exit 2; }
      tail_lines="$2"
      shift 2
      ;;
    --follow|-f)
      follow="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${resource}" && -z "${container}" ]]; then
  printf 'Set a workload or a container name.\n' >&2
  usage >&2
  exit 2
fi
if [[ -n "${resource}" && "${resource}" != */* ]]; then
  resource="deployment/${resource}"
fi
if [[ -n "${resource}" && ! "${resource}" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]]; then
  printf 'Invalid workload/resource: %s\n' "${resource}" >&2
  exit 2
fi
if [[ ! "${namespace}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
  printf 'Invalid Kubernetes namespace: %s\n' "${namespace}" >&2
  exit 2
fi
if [[ -n "${container}" && ! "${container}" =~ ^[A-Za-z0-9._-]+$ ]]; then
  printf 'Invalid container name: %s\n' "${container}" >&2
  exit 2
fi
if [[ ! "${tail_lines}" =~ ^[0-9]+$ ]]; then
  printf 'Tail line count must be a non-negative integer: %s\n' "${tail_lines}" >&2
  exit 2
fi

if [[ -z "${resource}" ]]; then
  discovery_command="sudo k3s kubectl --namespace=${namespace} get deployments,statefulsets,daemonsets,jobs --output=json"
  resources_json="$(bash "${SCRIPT_DIR}/ssh-machine.sh" app --command "${discovery_command}")"
  matches="$(python3 -c '
import json
import sys

requested = sys.argv[1]
resource_kinds = {
    "Deployment": "deployment",
    "StatefulSet": "statefulset",
    "DaemonSet": "daemonset",
    "Job": "job",
}
document = json.load(sys.stdin)
found = set()
for item in document.get("items", []):
    kind = resource_kinds.get(item.get("kind"))
    name = item.get("metadata", {}).get("name")
    pod_spec = item.get("spec", {}).get("template", {}).get("spec", {})
    containers = pod_spec.get("containers", []) + pod_spec.get("initContainers", [])
    if kind and name and any(container.get("name") == requested for container in containers):
        found.add(f"{kind}/{name}")
print("\n".join(sorted(found)))
' "${container}" <<< "${resources_json}")"
  if [[ -z "${matches}" ]]; then
    printf 'No workload in namespace %s contains container %s.\n' "${namespace}" "${container}" >&2
    exit 1
  fi
  mapfile -t matching_resources <<< "${matches}"
  if ((${#matching_resources[@]} > 1)); then
    printf 'Container %s matches multiple workloads in namespace %s:\n' "${container}" "${namespace}" >&2
    printf '  %s\n' "${matching_resources[@]}" >&2
    printf 'Pass WORKLOAD to choose one.\n' >&2
    exit 2
  fi
  resource="${matching_resources[0]}"
  resource_inferred="true"
fi

if [[ "${resource_inferred}" == "true" ]]; then
  printf 'Container %s matched to %s.\n' "${container}" "${resource}"
fi

remote_command="sudo k3s kubectl --namespace=${namespace} logs ${resource} --tail=${tail_lines}"
if [[ -n "${container}" ]]; then
  remote_command+=" --container=${container}"
else
  remote_command+=" --all-containers=true"
fi
if [[ "${follow}" == "true" ]]; then
  remote_command+=" --follow"
fi

exec bash "${SCRIPT_DIR}/ssh-machine.sh" app --command "${remote_command}"
