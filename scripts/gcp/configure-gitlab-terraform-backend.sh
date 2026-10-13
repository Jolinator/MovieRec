#!/usr/bin/env bash
set -euo pipefail

: "${TF_HTTP_ADDRESS:?Set TF_HTTP_ADDRESS before configuring the GitLab backend.}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
BACKEND_FILE="${REPO_ROOT}/deploy/gcp/terraform/backend.gitlab.tf"

cat > "${BACKEND_FILE}" <<'EOF'
# Generated for GitLab CI. Do not commit this file.
terraform {
  backend "http" {}
}
EOF
