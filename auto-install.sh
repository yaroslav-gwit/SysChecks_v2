#!/bin/bash

# SysChecks Auto-Installation Script
# Automatically downloads and installs the latest release

set -e

# Configuration
REPO="yaroslav-gwit/SysChecks_v2"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="syschecks"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_usage() {
    echo "SysChecks Auto-Installation Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --version VERSION    Install specific version (e.g., 1.0.0)"
    echo "  --dir DIR           Install to custom directory (default: $INSTALL_DIR)"
    echo "  --variant VARIANT   Choose binary variant:"
    echo "                        auto (default) - Auto-detect best variant"
    echo "                        standard       - syschecks-linux-amd64"
    echo "                        ubuntu18       - syschecks-ubuntu18"
    echo "                        alpine         - syschecks-alpine"
    echo "  --help              Show this help"
    echo ""
    echo "Examples:"
    echo "  curl -fsSL https://raw.githubusercontent.com/$REPO/main/auto-install.sh | bash"
    echo "  $0 --version 1.0.0"
    echo "  $0 --dir ~/.local/bin --variant alpine"
}

detect_os() {
    local os=""
    local arch=""
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)  os="linux" ;;
        Darwin*) os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *) 
            echo -e "${RED}Unsupported OS: $(uname -s)${NC}"
            exit 1
            ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        i386|i686) arch="386" ;;
        armv7l) arch="arm" ;;
        *)
            echo -e "${RED}Unsupported architecture: $(uname -m)${NC}"
            exit 1
            ;;
    esac
    
    echo "$os-$arch"
}

get_latest_version() {
    echo -e "${BLUE}Getting latest version...${NC}"
    curl -s "https://api.github.com/repos/$REPO/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"v([^"]+)".*/\1/'
}

download_binary() {
    local version="$1"
    local variant="$2"
    local binary_name=""
    local download_url=""
    
    case "$variant" in
        "auto")
            local platform
            platform=$(detect_os)
            binary_name="syschecks-$platform"
            if [ "$platform" = "windows-amd64" ]; then
                binary_name="$binary_name.exe"
            fi
            ;;
        "standard")
            binary_name="syschecks-linux-amd64"
            ;;
        "ubuntu18")
            binary_name="syschecks-ubuntu18"
            ;;
        "alpine")
            binary_name="syschecks-alpine"
            ;;
        *)
            echo -e "${RED}Unknown variant: $variant${NC}"
            exit 1
            ;;
    esac
    
    download_url="https://github.com/$REPO/releases/download/v$version/$binary_name"
    
    echo -e "${BLUE}Downloading $binary_name v$version...${NC}"
    echo -e "${YELLOW}URL: $download_url${NC}"
    
    if ! curl -fsSL "$download_url" -o "/tmp/$binary_name"; then
        echo -e "${RED}Failed to download binary${NC}"
        echo "Please check if version $version exists and the binary is available"
        exit 1
    fi
    
    echo "/tmp/$binary_name"
}

install_binary() {
    local temp_binary="$1"
    local install_path="$INSTALL_DIR/$BINARY_NAME"
    
    echo -e "${BLUE}Installing to $install_path...${NC}"
    
    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        echo -e "${YELLOW}Creating directory $INSTALL_DIR...${NC}"
        sudo mkdir -p "$INSTALL_DIR"
    fi
    
    # Install binary
    sudo cp "$temp_binary" "$install_path"
    sudo chmod +x "$install_path"
    
    # Cleanup
    rm -f "$temp_binary"
    
    echo -e "${GREEN}✓ Successfully installed $BINARY_NAME to $install_path${NC}"
}

verify_installation() {
    echo -e "${BLUE}Verifying installation...${NC}"
    
    if command -v "$BINARY_NAME" &> /dev/null; then
        local version
        version=$("$BINARY_NAME" version 2>/dev/null || echo "unknown")
        echo -e "${GREEN}✓ $BINARY_NAME is installed and working${NC}"
        echo -e "${BLUE}Version: $version${NC}"
        
        echo ""
        echo -e "${GREEN}Usage examples:${NC}"
        echo "  $BINARY_NAME kernel              # Check kernel status"
        echo "  $BINARY_NAME cleanup --keep 4    # Clean old kernels"
        echo "  $BINARY_NAME updates             # Check system updates"
        echo "  $BINARY_NAME --help              # Show all commands"
    else
        echo -e "${RED}✗ Installation verification failed${NC}"
        echo "You may need to add $INSTALL_DIR to your PATH"
        exit 1
    fi
}

# Parse arguments
VERSION=""
VARIANT="auto"

while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --variant)
            VARIANT="$2"
            shift 2
            ;;
        --help|-h)
            print_usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            print_usage
            exit 1
            ;;
    esac
done

# Main execution
echo -e "${GREEN}SysChecks Auto-Installation Script${NC}"
echo ""

# Get version if not specified
if [ -z "$VERSION" ]; then
    VERSION=$(get_latest_version)
    if [ -z "$VERSION" ]; then
        echo -e "${RED}Failed to get latest version${NC}"
        exit 1
    fi
fi

echo -e "${BLUE}Installing SysChecks v$VERSION...${NC}"
echo -e "${BLUE}Variant: $VARIANT${NC}"
echo -e "${BLUE}Install directory: $INSTALL_DIR${NC}"
echo ""

# Download and install
TEMP_BINARY=$(download_binary "$VERSION" "$VARIANT")
install_binary "$TEMP_BINARY"
verify_installation

echo ""
echo -e "${GREEN}🎉 Installation completed successfully!${NC}"
