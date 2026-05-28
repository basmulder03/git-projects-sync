#!/bin/sh
# git-sync installer — downloads the latest release and runs 'git-sync install'.
set -e

REPO="basmulder03/git-projects-sync"
BIN="git-sync"
INSTALL_DIR="${HOME}/.local/bin"

# Detect OS and architecture.
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *) echo "Unsupported architecture: ${ARCH}" >&2; exit 1 ;;
esac

# Resolve latest version from GitHub API.
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed -E 's/.*"v?([^"]+)".*/\1/')
if [ -z "${VERSION}" ]; then
  echo "Could not determine latest version." >&2; exit 1
fi

FILENAME="git-sync_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

echo "Downloading git-sync ${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

curl -fsSL "${URL}" | tar -xz -C "${TMP}"

mkdir -p "${INSTALL_DIR}"
install -m 755 "${TMP}/${BIN}" "${INSTALL_DIR}/${BIN}"

echo "Installed to ${INSTALL_DIR}/${BIN}"
echo ""
"${INSTALL_DIR}/${BIN}" install
