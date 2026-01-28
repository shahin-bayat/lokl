#!/bin/bash
set -e

REPO="shahin-bayat/lokl"
BINARY_NAME="lokl"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}$1${NC}"; }
warn() { echo -e "${YELLOW}$1${NC}"; }
error() { echo -e "${RED}$1${NC}" >&2; exit 1; }

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux|darwin) ;;
    *) error "Unsupported OS: $OS (only linux and darwin supported)" ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) error "Unsupported architecture: $ARCH" ;;
esac

# Get version (from argument or latest)
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    info "Fetching latest version..."
    VERSION=$(curl -sI "https://github.com/$REPO/releases/latest" | grep -i "^location:" | sed 's/.*tag\///' | tr -d '\r\n')
    if [ -z "$VERSION" ]; then
        error "Failed to fetch latest version"
    fi
fi
info "Installing lokl $VERSION"

# Determine install directory
if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
else
    mkdir -p "$HOME/.local/bin"
    INSTALL_DIR="$HOME/.local/bin"
    warn "Installing to $INSTALL_DIR (add to PATH if needed)"
fi

# Download URL
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY_NAME}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
info "Downloading from $DOWNLOAD_URL"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Download and extract
if ! curl -sL "$DOWNLOAD_URL" | tar xz -C "$TMP_DIR" 2>/dev/null; then
    error "Failed to download. Check if version $VERSION exists and has binaries."
fi

# Install binary
if [ -f "$TMP_DIR/$BINARY_NAME" ]; then
    mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
else
    error "Binary not found in archive"
fi

# Verify
if command -v lokl &>/dev/null; then
    info "lokl $VERSION installed successfully to $INSTALL_DIR"
    lokl --version
else
    warn "lokl installed to $INSTALL_DIR"
    warn "Add $INSTALL_DIR to your PATH to use it"
fi
