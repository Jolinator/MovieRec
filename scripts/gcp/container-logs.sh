#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
TF_DIR="${REPO_ROOT}/deploy/gcp/terraform"

usage() {
  printf 'Usage: %s WORKLOAD_OR_RESOURCE [--container NAME] [--namespace NAME] [--tail LINES] [--follow]\n' "${0##*/}"
  printf 'A name without a resource kind is treated as a Deployment.\n'
  printf 'Examples: %s llama-generation --container llama-server --follow\n' "${0##*/}"
  printf '          %s statefulset/movies-postgres --namespace movierec\n' "${0##*/}"
}

if (($# == 0)); then
  usage >&2
  exit 2
fi

resource="$1"
shift
if [[ "${resource}" != */* ]]; then
  resource="deployment/${resource}"
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

if [[ ! "${resource}" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]]; then
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

for command in terraform gcloud; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 2
  fi
done

app_instance="$(terraform -chdir="${TF_DIR}" output -raw app_instance_name)"
app_zone="$(terraform -chdir="${TF_DIR}" output -raw zone)"

remote_command="sudo k3s kubectl --namespace=${namespace} logs ${resource} --tail=${tail_lines}"
if [[ -n "${container}" ]]; then
  remote_command+=" --container=${container}"
else
  remote_command+=" --all-containers=true"
fi
if [[ "${follow}" == "true" ]]; then
  remote_command+=" --follow"
fi

exec gcloud compute ssh "${app_instance}" \
  --zone="${app_zone}" \
  --tunnel-through-iap \
  --quiet \
  --command="${remote_command}"
