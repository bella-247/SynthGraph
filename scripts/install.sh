#!/usr/bin/env bash
# SynthGraph Installer — Linux / macOS
# Usage: curl -sSf https://raw.githubusercontent.com/bella-247/SynthGraph/main/scripts/install.sh | sh
# Or:   ./scripts/install.sh

set -euo pipefail

REPO="bella-247/SynthGraph"
VERSION="${1:-latest}"

# ── 1. Detect platform ────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# ── 2. Resolve latest version ────────────────────────────────────
if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
  if [ -z "$VERSION" ]; then
    echo "Failed to resolve latest version"
    exit 1
  fi
fi

# ── 3. Try to download pre-built binary ──────────────────────────
BINARY_NAME="synthgraph-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY_NAME"

INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"

if curl -sL --fail "$DOWNLOAD_URL" -o "/tmp/synthgraph" 2>/dev/null; then
  chmod +x "/tmp/synthgraph"
  mv "/tmp/synthgraph" "$INSTALL_DIR/synthgraph"
  echo "Installed SynthGraph $VERSION ($OS/$ARCH) to $INSTALL_DIR/synthgraph"
  echo ""
  echo "Make sure $INSTALL_DIR is in your PATH:"
  echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
  echo ""
  echo "Or add that line to ~/.bashrc / ~/.zshrc to make it permanent."
  exit 0
fi

# ── 4. Fallback: build from source ────────────────────────────────
if ! command -v go &>/dev/null; then
  echo "No pre-built binary for $OS/$ARCH, and Go is not installed."
  echo ""
  echo "Install Go: https://go.dev/dl/"
  echo "Then re-run this script, or build manually:"
  echo "  git clone https://github.com/$REPO.git"
  echo "  cd SynthGraph"
  echo "  CGO_ENABLED=1 go build -o \$HOME/.local/bin/synthgraph ./cmd/synthgraph/"
  exit 1
fi

if ! command -v gcc &>/dev/null; then
  echo "Go is installed, but GCC is required for CGO (PostgreSQL parser)."
  echo ""
  echo "macOS: xcode-select --install"
  echo "Linux: sudo apt install gcc libpq-dev   (or your distro equivalent)"
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Building SynthGraph from source (this takes a minute)..."
git clone -q "https://github.com/$REPO.git" "$TMP_DIR/src"

CGO_ENABLED=1 go build -o "$TMP_DIR/synthgraph" "$TMP_DIR/src/cmd/synthgraph/"
CGO_ENABLED=1 go build -o "$TMP_DIR/synthgraph-web" "$TMP_DIR/src/cmd/synthgraph-web/"

mv "$TMP_DIR/synthgraph" "$INSTALL_DIR/synthgraph"
mv "$TMP_DIR/synthgraph-web" "$INSTALL_DIR/synthgraph-web"

echo "Installed to $INSTALL_DIR/synthgraph"
echo ""
echo "Make sure $INSTALL_DIR is in your PATH:"
echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
