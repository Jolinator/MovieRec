#!/usr/bin/env bash

# Source this file so the Terraform backend variables remain in the current shell.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  printf 'Source this script from Bash: source scripts/gcp/export-gitlab-terraform-env.sh\n' >&2
  exit 2
fi

_export_gitlab_terraform_env() {
  local script_dir repo_root remote_url remote_path remote_authority api_root scheme
  local encoded_project_path project_id response gitlab_username gitlab_token

  script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)" || return 1
  repo_root="$(cd -- "${script_dir}/../.." && pwd)" || return 1

  if ! remote_url="$(git -c "safe.directory=${repo_root}" -C "${repo_root}" remote get-url origin 2>/dev/null)"; then
    printf 'Could not read the origin Git remote in %s.\n' "${repo_root}" >&2
    return 1
  fi

  case "${remote_url}" in
    https://*)
      scheme="https"
      remote_authority="${remote_url#*://}"
      remote_authority="${remote_authority%%/*}"
      remote_authority="${remote_authority##*@}"
      api_root="${scheme}://${remote_authority}"
      remote_path="${remote_url#*://*/}"
      ;;
    http://*)
      printf 'The origin remote must use HTTPS or SSH to protect the API token.\n' >&2
      return 1
      ;;
    ssh://*)
      remote_authority="${remote_url#ssh://}"
      remote_authority="${remote_authority%%/*}"
      remote_authority="${remote_authority##*@}"
      api_root="https://${remote_authority}"
      remote_path="${remote_url#ssh://*/}"
      ;;
    *@*:* )
      remote_authority="${remote_url%%:*}"
      remote_authority="${remote_authority##*@}"
      api_root="https://${remote_authority}"
      remote_path="${remote_url#*:}"
      ;;
    *)
      printf 'Unsupported origin remote URL format: %s\n' "${remote_url}" >&2
      return 1
      ;;
  esac

  remote_path="${remote_path%.git}"
  remote_path="${remote_path%/}"
  if [[ -z "${remote_path}" || "${remote_path}" == "${remote_url}" ]]; then
    printf 'Could not determine the GitLab project path from origin: %s\n' "${remote_url}" >&2
    return 1
  fi
  encoded_project_path="${remote_path//\//%2F}"

  gitlab_username="${GITLAB_USERNAME:-${TF_HTTP_USERNAME:-}}"
  if [[ -z "${gitlab_username}" ]]; then
    IFS= read -r -p 'GitLab username: ' gitlab_username || return 1
  fi

  gitlab_token="${GITLAB_TOKEN:-${TF_HTTP_PASSWORD:-}}"
  if [[ -z "${gitlab_token}" ]]; then
    IFS= read -r -s -p 'GitLab API token (input hidden): ' gitlab_token || return 1
    printf '\n' >&2
  fi
  if [[ -z "${gitlab_username}" || -z "${gitlab_token}" ]]; then
    printf 'A GitLab username and API token are required.\n' >&2
    return 1
  fi

  if ! command -v curl >/dev/null 2>&1; then
    printf 'Required command not found: curl\n' >&2
    return 1
  fi
  if ! command -v python3 >/dev/null 2>&1; then
    printf 'Required command not found: python3 (needed to read the GitLab API response).\n' >&2
    return 1
  fi

  printf 'Looking up project ID for %s on %s...\n' "${remote_path}" "${api_root}"
  if ! response="$(curl --fail --silent --show-error \
    --header "PRIVATE-TOKEN: ${gitlab_token}" \
    "${api_root}/api/v4/projects/${encoded_project_path}")"; then
    printf 'GitLab project lookup failed. Check the token API scope and project access.\n' >&2
    return 1
  fi
  if ! project_id="$(printf '%s' "${response}" | python3 -c 'import json, sys; print(json.load(sys.stdin)["id"])' 2>/dev/null)"; then
    printf 'GitLab returned a response without a readable project ID.\n' >&2
    return 1
  fi

  export GITLAB_PROJECT_ID="${project_id}"
  export TF_HTTP_ADDRESS="${api_root}/api/v4/projects/${project_id}/terraform/state/movierec"
  export TF_HTTP_LOCK_ADDRESS="${TF_HTTP_ADDRESS}/lock"
  export TF_HTTP_UNLOCK_ADDRESS="${TF_HTTP_ADDRESS}/lock"
  export TF_HTTP_USERNAME="${gitlab_username}"
  export TF_HTTP_PASSWORD="${gitlab_token}"
  export TF_HTTP_LOCK_METHOD=POST
  export TF_HTTP_UNLOCK_METHOD=DELETE
  export TF_HTTP_RETRY_WAIT_MIN=5

  printf 'Exported Terraform HTTP backend settings for GitLab project ID %s.\n' "${project_id}"
}

if _export_gitlab_terraform_env; then
  _export_gitlab_terraform_env_status=0
else
  _export_gitlab_terraform_env_status=$?
fi
unset -f _export_gitlab_terraform_env
return "${_export_gitlab_terraform_env_status}"
