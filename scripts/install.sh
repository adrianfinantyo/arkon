#!/usr/bin/env bash
set -euo pipefail

# Arkon installation script
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/adrianfinantyo/arkon/main/scripts/install.sh | bash
# Or with specific version:
#   ARKON_VERSION=v0.1.0 curl -fsSL ... | bash

REPO="adrianfinantyo/arkon"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Check operating system
OS="$(uname -s)"
if [ "$OS" != "Linux" ]; then
  echo "Error: Arkon currently supports Linux only (detected: $OS)." >&2
  exit 1
fi

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "Error: Unsupported architecture $ARCH. Arkon builds are available for amd64 and arm64." >&2
    exit 1
  ;;
esac

# Resolve version
if [ -n "${ARKON_VERSION:-}" ]; then
  TAG="$ARKON_VERSION"
  case "$TAG" in
    v*) ;;
    *) TAG="v$TAG" ;;
  esac
else
  echo "Fetching latest release tag for $REPO..."
  TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
  if [ -z "$TAG" ]; then
    echo "Error: Could not resolve latest release from GitHub API. Set ARKON_VERSION manually to bypass." >&2
    exit 1
  fi
fi

VERSION="${TAG#v}"
TARBALL="arkon_${VERSION}_linux_${ARCH}.tar.gz"
CHECKSUMS="checksums.txt"
RELEASE_URL="https://github.com/$REPO/releases/download/$TAG"

echo "Installing Arkon $TAG (linux/$ARCH)..."

# Temporary directory for download and extraction
TMP_DIR="$(mktemp -d -t arkon-install.XXXXXX)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo "Downloading $TARBALL..."
if ! curl -fsSL -o "$TMP_DIR/$TARBALL" "$RELEASE_URL/$TARBALL"; then
  echo "Error: Failed to download $RELEASE_URL/$TARBALL." >&2
  exit 1
fi

echo "Downloading $CHECKSUMS..."
if ! curl -fsSL -o "$TMP_DIR/$CHECKSUMS" "$RELEASE_URL/$CHECKSUMS"; then
  echo "Error: Failed to download $RELEASE_URL/$CHECKSUMS." >&2
  exit 1
fi

echo "Verifying SHA-256 checksum..."
(
  cd "$TMP_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    grep "$TARBALL" "$CHECKSUMS" | sha256sum -c --status
  elif command -v shasum >/dev/null 2>&1; then
    grep "$TARBALL" "$CHECKSUMS" | shasum -a 256 -c --status
  else
    echo "Error: Neither sha256sum nor shasum is installed on this host." >&2
    exit 1
  fi
)

echo "Extracting binary..."
tar -xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR" arkon

TARGET="$INSTALL_DIR/arkon"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/arkon" "$TARGET"
  chmod 0755 "$TARGET"
else
  echo "Destination $INSTALL_DIR requires elevated privileges. Elevating with sudo..."
  sudo mv "$TMP_DIR/arkon" "$TARGET"
  sudo chmod 0755 "$TARGET"
fi

echo "Installed Arkon to $TARGET"
