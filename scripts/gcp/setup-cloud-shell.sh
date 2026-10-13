#!/usr/bin/env bash
set -euo pipefail

CUSTOMIZE_FILE="${HOME}/.customize_environment"
MARKER="# BEGIN MovieRec Cloud Shell tools v2"
BLOCK_FILE="$(mktemp)"
trap 'rm -f -- "${BLOCK_FILE}"' EXIT

cat > "${BLOCK_FILE}" <<'EOF'

# BEGIN MovieRec Cloud Shell tools v2
set -e

if ! command -v micro >/dev/null 2>&1 || ! command -v terraform >/dev/null 2>&1; then
  apt-get update
  apt-get install -y ca-certificates gnupg wget

  HASHICORP_KEYRING="/usr/share/keyrings/hashicorp-archive-keyring.gpg"
  HASHICORP_KEY_TEMP="$(mktemp)"
  wget -qO "${HASHICORP_KEY_TEMP}" https://apt.releases.hashicorp.com/gpg
  gpg --dearmor --yes -o "${HASHICORP_KEYRING}" "${HASHICORP_KEY_TEMP}"
  rm -f -- "${HASHICORP_KEY_TEMP}"

  . /etc/os-release
  DISTRO_CODENAME="${UBUNTU_CODENAME:-${VERSION_CODENAME}}"
  printf 'deb [arch=%s signed-by=%s] https://apt.releases.hashicorp.com %s main\n' \
    "$(dpkg --print-architecture)" "${HASHICORP_KEYRING}" "${DISTRO_CODENAME}" \
    > /etc/apt/sources.list.d/hashicorp.list

  apt-get update
  apt-get install -y micro terraform
fi

if ! command -v ansible-playbook >/dev/null 2>&1; then
  apt-get update
  apt-get install -y ansible
fi

terraform version
micro --version
ansible --version | head -n 1
echo "Cloud Shell setup complete: Micro, Terraform and Ansible are installed."
# END MovieRec Cloud Shell tools v2
EOF

if [[ ! -f "${CUSTOMIZE_FILE}" ]]; then
  printf '%s\n' '#!/usr/bin/env bash' > "${CUSTOMIZE_FILE}"
fi

if ! grep -Fq "${MARKER}" "${CUSTOMIZE_FILE}"; then
  cat "${BLOCK_FILE}" >> "${CUSTOMIZE_FILE}"
fi
chmod +x "${CUSTOMIZE_FILE}"

printf 'Persistent setup saved to %s\n' "${CUSTOMIZE_FILE}"
printf 'Installing Micro, Terraform and Ansible in this Cloud Shell session...\n'
sudo bash "${BLOCK_FILE}"

# NumPy can improve IAP TCP upload bandwidth. It is an optional optimization,
# so keep setup working if the Cloud SDK's Python cannot install it.
if command -v gcloud >/dev/null 2>&1; then
  if GCLOUD_PYTHON_PATH="$(gcloud info --format='value(basic.python_location)' 2>/dev/null)" \
    && [[ -n "${GCLOUD_PYTHON_PATH}" ]]; then
    if ! "${GCLOUD_PYTHON_PATH}" -c 'import numpy' >/dev/null 2>&1; then
      if ! "${GCLOUD_PYTHON_PATH}" -m pip install numpy; then
        printf '%s\n' 'Optional NumPy install failed; see https://cloud.google.com/iap/docs/using-tcp-forwarding#increasing_the_tcp_upload_bandwidth' >&2
      fi
    fi

    if ! grep -Fq 'CLOUDSDK_PYTHON_SITEPACKAGES=1' "${CUSTOMIZE_FILE}"; then
      printf '\n%s\n' '# Let gcloud import NumPy for IAP TCP upload acceleration.' \
        'export CLOUDSDK_PYTHON_SITEPACKAGES=1' >> "${CUSTOMIZE_FILE}"
    fi
    export CLOUDSDK_PYTHON_SITEPACKAGES=1
  else
    printf '%s\n' 'Could not locate gcloud Python; skipping optional NumPy install.' >&2
  fi
fi
