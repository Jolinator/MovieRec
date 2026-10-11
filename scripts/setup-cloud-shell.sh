#!/usr/bin/env bash
set -euo pipefail

CUSTOMIZE_FILE="${HOME}/.customize_environment"
MARKER="# BEGIN MovieRec Cloud Shell tools"
BLOCK_FILE="$(mktemp)"
trap 'rm -f -- "${BLOCK_FILE}"' EXIT

cat > "${BLOCK_FILE}" <<'EOF'

# BEGIN MovieRec Cloud Shell tools
set -e

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
terraform version
micro --version
echo "Cloud Shell setup complete: Micro and Terraform are installed."
# END MovieRec Cloud Shell tools
EOF

if [[ ! -f "${CUSTOMIZE_FILE}" ]]; then
  printf '%s\n' '#!/usr/bin/env bash' > "${CUSTOMIZE_FILE}"
fi

if ! grep -Fq "${MARKER}" "${CUSTOMIZE_FILE}"; then
  cat "${BLOCK_FILE}" >> "${CUSTOMIZE_FILE}"
fi
chmod +x "${CUSTOMIZE_FILE}"

printf 'Persistent setup saved to %s\n' "${CUSTOMIZE_FILE}"
printf 'Installing Micro and Terraform in this Cloud Shell session...\n'
sudo bash "${BLOCK_FILE}"
