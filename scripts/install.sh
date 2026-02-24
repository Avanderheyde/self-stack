#!/bin/sh
set -e

REPO="Avanderheyde/self-stack"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest release tag
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "Failed to fetch latest release"
  exit 1
fi

ASSET="selfstack_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

echo "Downloading SelfStack ${TAG} for ${OS}/${ARCH}..."

TMP=$(mktemp -d)
curl -fsSL "$URL" -o "${TMP}/${ASSET}"
tar -xzf "${TMP}/${ASSET}" -C "$TMP"

# Install binary
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  echo "Installing to ${INSTALL_DIR} (add to PATH if needed)"
fi

mv "${TMP}/selfstack" "${INSTALL_DIR}/selfstack"
chmod +x "${INSTALL_DIR}/selfstack"
rm -rf "$TMP"

echo "SelfStack ${TAG} installed to ${INSTALL_DIR}/selfstack"
selfstack status 2>/dev/null || echo "Run 'selfstack serve' to start."
