#!/usr/bin/env sh

set -e

# Bumper install script
# Usage: curl -fsSL https://raw.githubusercontent.com/disintegrator/bumper/main/install.sh | sh

GITHUB_REPO="disintegrator/bumper"
VERSION_URL="https://raw.githubusercontent.com/$GITHUB_REPO/refs/heads/main/VERSION"
INSTALL_DIR="/usr/local/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    printf "${GREEN}==>${NC} %s\n" "$1"
}

error() {
    printf "${RED}Error:${NC} %s\n" "$1" >&2
    exit 1
}

warn() {
    printf "${YELLOW}Warning:${NC} %s\n" "$1" >&2
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux";;
        Darwin*)    echo "darwin";;
        *)          error "Unsupported operating system: $(uname -s)";;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64";;
        aarch64|arm64)  echo "arm64";;
        i386|i686)      echo "386";;
        *)              error "Unsupported architecture: $(uname -m)";;
    esac
}

# Determine download tool (curl or wget)
get_download_tool() {
    if command -v curl >/dev/null 2>&1; then
        echo "curl"
    elif command -v wget >/dev/null 2>&1; then
        echo "wget"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download a file
download() {
    _url="$1"
    _output="$2"
    _tool="$3"

    if [ "$_tool" = "curl" ]; then
        curl -fsSL "$_url" -o "$_output" || error "Failed to download $_url"
    else
        wget -q "$_url" -O "$_output" || error "Failed to download $_url"
    fi
}

# Download a file to stdout
download_stdout() {
    _url="$1"
    _tool="$2"

    if [ "$_tool" = "curl" ]; then
        curl -fsSL "$_url" || error "Failed to download $_url"
    else
        wget -qO- "$_url" || error "Failed to download $_url"
    fi
}

# Main installation logic
main() {
    info "Starting bumper installation..."

    # Detect system
    OS=$(detect_os)
    ARCH=$(detect_arch)
    DOWNLOAD_TOOL=$(get_download_tool)

    info "Detected OS: $OS"
    info "Detected architecture: $ARCH"
    info "Using download tool: $DOWNLOAD_TOOL"

    # Get latest version
    info "Fetching latest version..."
    VERSION=$(download_stdout "$VERSION_URL" "$DOWNLOAD_TOOL" | tr -d '[:space:]')

    if [ -z "$VERSION" ]; then
        error "Failed to fetch version from $VERSION_URL"
    fi

    info "Latest version: $VERSION"

    # Construct download URL
    ARCHIVE="bumper_${VERSION}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${ARCHIVE}"

    info "Download URL: $DOWNLOAD_URL"

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    # Download archive
    info "Downloading bumper..."
    download "$DOWNLOAD_URL" "$TMP_DIR/$ARCHIVE" "$DOWNLOAD_TOOL"

    # Extract archive
    info "Extracting archive..."
    tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR" || error "Failed to extract archive"

    # Check if we need sudo for installation
    if [ -w "$INSTALL_DIR" ]; then
        SUDO=""
    else
        if command -v sudo >/dev/null 2>&1; then
            warn "Installation requires sudo access to write to $INSTALL_DIR"
            SUDO="sudo"
        else
            error "Cannot write to $INSTALL_DIR and sudo is not available"
        fi
    fi

    # Install binary
    info "Installing bumper to $INSTALL_DIR..."
    $SUDO mv "$TMP_DIR/bumper" "$INSTALL_DIR/bumper" || error "Failed to install bumper"
    $SUDO chmod +x "$INSTALL_DIR/bumper" || error "Failed to set executable permissions"

    # Verify installation
    info "Installation successful!"
    info "Verifying installation..."
    echo ""
    bumper --version || error "Failed to run bumper --version"
    echo ""
    info "bumper has been installed successfully!"
}

main