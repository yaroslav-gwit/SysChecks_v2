#!/bin/bash

# GitHub Release Script using GitHub CLI
# Requires: gh CLI tool installed and authenticated

set -e

# Configuration
REPO="yaroslav-gwit/SysChecks_v2"
# Get version from git tags if not provided
if [ -z "$1" ]; then
    VERSION=$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || date +%Y.%m.%d)
else
    VERSION="$1"
fi
RELEASE_NAME="SysChecks v$VERSION"
BUILD_DIR="./bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_usage() {
    echo "Usage: $0 [VERSION] [RELEASE_TYPE]"
    echo ""
    echo "VERSION: Release version (default: YYYY.MM.DD)"
    echo "RELEASE_TYPE: 'draft', 'prerelease', or 'release' (default: release)"
    echo ""
    echo "Examples:"
    echo "  $0 1.0.0 release"
    echo "  $0 1.0.0-beta prerelease"
    echo "  $0 # Uses current date as version"
    echo ""
    echo "Prerequisites:"
    echo "  - GitHub CLI installed: https://cli.github.com/"
    echo "  - Authenticated: gh auth login"
}

check_prerequisites() {
    echo -e "${BLUE}Checking prerequisites...${NC}"
    
    # Check if gh is installed
    if ! command -v gh &> /dev/null; then
        echo -e "${RED}Error: GitHub CLI (gh) is not installed${NC}"
        echo "Install it from: https://cli.github.com/"
        exit 1
    fi
    
    # Check if authenticated
    if ! gh auth status &> /dev/null; then
        echo -e "${RED}Error: Not authenticated with GitHub${NC}"
        echo "Run: gh auth login"
        exit 1
    fi
    
    echo -e "${GREEN}✓ GitHub CLI is installed and authenticated${NC}"
}

build_all_binaries() {
    echo -e "${BLUE}Building all binaries...${NC}"
    
    # Build cross-platform binaries
    # ./build-advanced.sh cross
    
    # Build Docker variants for Linux
    sudo ./build-advanced.sh docker ubuntu18
    rm -f ./bin/syschecks-linux-amd64
    mv ./bin/syschecks-ubuntu18 ./bin/syschecks-linux-amd64

    # Build for Alpine
    # ./build-advanced.sh docker alpine
    
    echo -e "${GREEN}✓ All binaries built${NC}"
}

create_release() {
    local version="$1"
    local release_type="${2:-release}"
    local release_flags=""
    
    case "$release_type" in
        "draft")
            release_flags="--draft"
            ;;
        "prerelease")
            release_flags="--prerelease"
            ;;
        "release")
            release_flags=""
            ;;
        *)
            echo -e "${RED}Invalid release type: $release_type${NC}"
            echo "Use: draft, prerelease, or release"
            exit 1
            ;;
    esac
    
    echo -e "${BLUE}Creating GitHub release v$version...${NC}"
    
    # Generate release notes
    local release_notes="## SysChecks v$version

### Features
- Kernel reboot checks with JSON output
- Kernel cleanup functionality with --keep flag
- System update checks for multiple package managers
- Cross-platform binary support

### Binaries Included
- **Linux AMD64**: syschecks-linux-amd64 (Ubuntu 18.04 compatible)


### Installation
\`\`\`bash
# Download and install (Linux AMD64)
wget https://github.com/$REPO/releases/download/v$version/syschecks-linux-amd64
chmod +x syschecks-linux-amd64
sudo mv syschecks-linux-amd64 /usr/local/bin/syschecks

# Or use the install script
curl -fsSL https://raw.githubusercontent.com/$REPO/main/install.sh | bash
\`\`\`

### Usage
\`\`\`bash
# Check kernel status
syschecks kernel

# Clean up old kernels (keep 4 newest)
syschecks cleanup --keep 4

# Check for system updates
syschecks updates
\`\`\`"
    
    # Create the release
    gh release create "v$version" \
        --repo "$REPO" \
        --title "$RELEASE_NAME" \
        --notes "$release_notes" \
        $release_flags \
        "$BUILD_DIR"/syschecks-* \
        || {
            echo -e "${RED}Failed to create release${NC}"
            exit 1
        }
    
    echo -e "${GREEN}✓ Release v$version created successfully!${NC}"
    echo -e "${YELLOW}View at: https://github.com/$REPO/releases/tag/v$version${NC}"
}

# Main execution
case "${1:-}" in
    "-h"|"--help"|"help")
        print_usage
        exit 0
        ;;
esac

check_prerequisites
build_all_binaries
create_release "$VERSION" "${2:-release}"

echo ""
echo -e "${GREEN}🎉 Release published successfully!${NC}"
echo -e "${BLUE}Next steps:${NC}"
echo "1. Update documentation if needed"
echo "2. Announce the release"
echo "3. Update package managers (if applicable)"
