#!/bin/bash
#
# Claude Token Monitor - Auto Install Script (macOS/Linux)
# This script is triggered by Claude Code plugin post-install hook
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REPO="young1lin/claude-token-monitor"
INSTALL_DIR="$HOME/.claude"
BINARY_NAME="statusline"

echo -e "${GREEN}[claude-token-monitor]${NC} Starting auto-update..."

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    darwin) OS="darwin" ;;
    linux)  OS="linux" ;;
    *)
        echo -e "${RED}Error: Unsupported OS: $OS${NC}"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    amd64)   ARCH="amd64" ;;
    arm64)   ARCH="arm64" ;;
    aarch64) ARCH="arm64" ;;
    *)
        echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

echo -e "${GREEN}[claude-token-monitor]${NC} Platform: ${OS}/${ARCH}"

# Get latest version from GitHub API
echo -e "${GREEN}[claude-token-monitor]${NC} Fetching latest version..."
LATEST_VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo -e "${RED}Error: Failed to get latest version${NC}"
    exit 1
fi

echo -e "${GREEN}[claude-token-monitor]${NC} Latest version: v${LATEST_VERSION}"

# Check current version
CURRENT_VERSION=""
if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    CURRENT_VERSION=$("$INSTALL_DIR/$BINARY_NAME" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
fi

if [ "$CURRENT_VERSION" = "$LATEST_VERSION" ]; then
    echo -e "${GREEN}[claude-token-monitor]${NC} Already up to date (v${CURRENT_VERSION})"
    exit 0
fi

if [ -n "$CURRENT_VERSION" ]; then
    echo -e "${YELLOW}[claude-token-monitor]${NC} Updating: v${CURRENT_VERSION} → v${LATEST_VERSION}"
else
    echo -e "${GREEN}[claude-token-monitor]${NC} Installing: v${LATEST_VERSION}"
fi

# Build download URL
ARCHIVE_NAME="statusline_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${LATEST_VERSION}/${ARCHIVE_NAME}"

echo -e "${GREEN}[claude-token-monitor]${NC} Downloading: ${DOWNLOAD_URL}"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Download
curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME"

if [ ! -f "$TMP_DIR/$ARCHIVE_NAME" ]; then
    echo -e "${RED}Error: Download failed${NC}"
    exit 1
fi

# Extract
echo -e "${GREEN}[claude-token-monitor]${NC} Extracting..."
tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"

# Find binary (might be in subdirectory)
BINARY_PATH=$(find "$TMP_DIR" -name "$BINARY_NAME" -type f | head -1)

if [ -z "$BINARY_PATH" ]; then
    echo -e "${RED}Error: Binary not found in archive${NC}"
    exit 1
fi

# Create install directory if not exists
mkdir -p "$INSTALL_DIR"

# Install
mv "$BINARY_PATH" "$INSTALL_DIR/$BINARY_NAME"
chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo -e "${GREEN}[claude-token-monitor]${NC} Installed to: $INSTALL_DIR/$BINARY_NAME"

# Verify
INSTALLED_VERSION=$("$INSTALL_DIR/$BINARY_NAME" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")
echo -e "${GREEN}[claude-token-monitor]${NC} Verified: v${INSTALLED_VERSION}"

echo -e "${GREEN}[claude-token-monitor]${NC} Update complete!"
