#!/bin/sh
set -e

REPO="${SELFSTACK_REPO:-Avanderheyde/self-stack}"
TAG="${SELFSTACK_VERSION:-}"

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

# Get latest release tag unless explicitly pinned.
if [ -z "$TAG" ]; then
  RELEASES_URL="https://api.github.com/repos/${REPO}/releases/latest"
  HTTP_CODE=$(curl -sS -o /tmp/selfstack-release.$$.json -w "%{http_code}" "$RELEASES_URL" || echo "000")
  if [ "$HTTP_CODE" != "200" ]; then
    cat >&2 <<EOF
No release found for ${REPO} (HTTP ${HTTP_CODE} from ${RELEASES_URL}).

Options:
  1. Use a fork that has releases:
       curl -fsSL https://raw.githubusercontent.com/Avanderheyde/self-stack/main/scripts/install.sh | SELFSTACK_REPO=owner/name sh
  2. Pin to a specific tag:
       curl -fsSL https://raw.githubusercontent.com/Avanderheyde/self-stack/main/scripts/install.sh | SELFSTACK_VERSION=vX.Y.Z sh
  3. Build from source:
       git clone https://github.com/${REPO}
       cd self-stack && make build && sudo mv selfstack /usr/local/bin/
EOF
    rm -f /tmp/selfstack-release.$$.json
    exit 1
  fi
  TAG=$(grep '"tag_name"' /tmp/selfstack-release.$$.json | head -1 | cut -d'"' -f4)
  rm -f /tmp/selfstack-release.$$.json
  if [ -z "$TAG" ]; then
    echo "Could not parse release tag from ${RELEASES_URL}" >&2
    exit 1
  fi
fi

ASSET="selfstack_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

echo "Downloading SelfStack ${TAG} for ${OS}/${ARCH}..."

TMP=$(mktemp -d)
if ! curl -fsSL "$URL" -o "${TMP}/${ASSET}"; then
  echo "Download failed: ${URL}" >&2
  rm -rf "$TMP"
  exit 1
fi
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
