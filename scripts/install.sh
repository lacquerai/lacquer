#!/bin/sh
set -e

# Lacquer installer script
# Usage: curl -sSL https://lacquer.ai/install.sh | sh

BINARY_NAME="laq"
REPO_OWNER="lacquerai"
REPO_NAME="lacquer"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# OS and architecture detection
OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
    Linux*)     PLATFORM=linux;;
    Darwin*)    PLATFORM=darwin;;
    MINGW*|MSYS*|CYGWIN*) PLATFORM=windows;;
    *)          echo "${RED}Unsupported operating system: ${OS}${NC}"; exit 1;;
esac

case "${ARCH}" in
    x86_64|amd64) ARCH=x86_64;;
    aarch64|arm64) ARCH=arm64;;
    i386|i686) ARCH=i386;;
    *) echo "${RED}Unsupported architecture: ${ARCH}${NC}"; exit 1;;
esac

# Check if running on Windows through WSL
if [ "${PLATFORM}" = "linux" ] && grep -qi microsoft /proc/version 2>/dev/null; then
    echo "${YELLOW}Detected WSL environment${NC}"
fi

# Function to get latest release version
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -s "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        echo "${RED}Error: curl or wget is required but not installed.${NC}" >&2
        exit 1
    fi
}

# Function to download file
download() {
    url=$1
    output=$2
    
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$output"
    else
        echo "${RED}Error: curl or wget is required but not installed.${NC}" >&2
        exit 1
    fi
}

echo "Installing Lacquer..."
echo "Platform: ${PLATFORM}"
echo "Architecture: ${ARCH}"

# Get the latest version
VERSION="${VERSION:-$(get_latest_version)}"
if [ -z "$VERSION" ]; then
    echo "${RED}Failed to determine latest version${NC}"
    exit 1
fi

echo "Version: ${VERSION}"

# Construct download URL
# Capitalize platform name for archive naming
case "${PLATFORM}" in
    linux)   PLATFORM_CAPITALIZED="Linux";;
    darwin)  PLATFORM_CAPITALIZED="Darwin";;
    windows) PLATFORM_CAPITALIZED="Windows";;
    *)       PLATFORM_CAPITALIZED="${PLATFORM}";;
esac

ARCHIVE_NAME="lacquer_${VERSION#v}_${PLATFORM_CAPITALIZED}_${ARCH}"
if [ "${PLATFORM}" = "windows" ]; then
    ARCHIVE_NAME="${ARCHIVE_NAME}.zip"
else
    ARCHIVE_NAME="${ARCHIVE_NAME}.tar.gz"
fi

DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${ARCHIVE_NAME}"

# Create temporary directory
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${DOWNLOAD_URL}..."
download "$DOWNLOAD_URL" "$TMP_DIR/$ARCHIVE_NAME"

# Extract archive
echo "Extracting..."
cd "$TMP_DIR"
if [ "${PLATFORM}" = "windows" ]; then
    unzip -q "$ARCHIVE_NAME"
else
    tar -xzf "$ARCHIVE_NAME"
fi

# Find the binary
BINARY_PATH="${TMP_DIR}/${BINARY_NAME}"
if [ "${PLATFORM}" = "windows" ]; then
    BINARY_PATH="${BINARY_PATH}.exe"
fi

if [ ! -f "$BINARY_PATH" ]; then
    echo "${RED}Binary not found in archive${NC}"
    exit 1
fi

# Check if we need sudo for installation
SUDO=""
if [ ! -w "$INSTALL_DIR" ] && [ "$INSTALL_DIR" = "/usr/local/bin" ]; then
    if command -v sudo >/dev/null 2>&1; then
        echo "${YELLOW}Installation requires sudo privileges${NC}"
        SUDO="sudo"
    else
        echo "${RED}Cannot write to $INSTALL_DIR and sudo is not available${NC}"
        echo "Please run as root or set INSTALL_DIR to a writable directory"
        exit 1
    fi
fi

# Create install directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
    echo "Creating installation directory: $INSTALL_DIR"
    $SUDO mkdir -p "$INSTALL_DIR"
fi

# Install the binary
echo "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
$SUDO mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY_NAME}"
$SUDO chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# Verify installation
if command -v "${BINARY_NAME}" >/dev/null 2>&1; then
    echo "${GREEN}Lacquer installed successfully!${NC}"
    echo ""
    echo "Run '${BINARY_NAME} --help' to get started"
    echo "Visit https://lacquer.ai for documentation"
else
    echo "${YELLOW}Installation complete, but ${BINARY_NAME} is not in your PATH${NC}"
    echo "Add ${INSTALL_DIR} to your PATH:"
    echo ""
    case "$SHELL" in
        */bash)
            echo "  echo 'export PATH=\"\$PATH:${INSTALL_DIR}\"' >> ~/.bashrc"
            echo "  source ~/.bashrc"
            ;;
        */zsh)
            echo "  echo 'export PATH=\"\$PATH:${INSTALL_DIR}\"' >> ~/.zshrc"
            echo "  source ~/.zshrc"
            ;;
        */fish)
            echo "  echo 'set -gx PATH \$PATH ${INSTALL_DIR}' >> ~/.config/fish/config.fish"
            echo "  source ~/.config/fish/config.fish"
            ;;
        *)
            echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
            ;;
    esac
fi