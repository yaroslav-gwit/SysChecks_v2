#!/bin/bash

# Comprehensive build script for SysChecks
# Supports multiple build methods for different use cases

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/bin"

# Get version information from git
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')

# Build flags with version information
LDFLAGS="-X 'syschecks/cmd.Version=$VERSION' -X 'syschecks/cmd.GitCommit=$COMMIT' -X 'syschecks/cmd.BuildDate=$DATE'"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_usage() {
    echo "Usage: $0 [METHOD] [BASE]"
    echo ""
    echo "Build methods:"
    echo "  docker      - Build using Docker (specify base image)"
    echo "  native      - Build natively on current system (fastest for development)"
    echo "  static      - Build static binary natively (good compromise)"
    echo "  cross       - Cross-compile for multiple architectures"
    echo ""
    echo "Docker base images (for docker method):"
    echo "  ubuntu18    - Ubuntu 18.04 LTS (glibc 2.27, good compatibility)"
    echo "  debian11    - Debian 11 Bullseye (glibc 2.31, current stable)"
    echo "  alpine      - Alpine Linux (musl libc, smallest binaries)"
    echo ""
    echo "Examples:"
    echo "  $0 docker ubuntu18"
    echo "  $0 docker alpine"
    echo "  $0 native"
    echo "  $0 static"
    echo "  $0 cross"
}

build_docker() {
    local base_image="${1:-ubuntu18}"
    local dockerfile="Dockerfile"
    local image_name="syschecks-builder"
    
    case "$base_image" in
        "ubuntu18")
            dockerfile="Dockerfile"
            echo -e "${BLUE}Building with Docker (Ubuntu 18.04 LTS - glibc 2.27)...${NC}"
            ;;
        "debian11")
            dockerfile="Dockerfile.debian11"
            echo -e "${BLUE}Building with Docker (Debian 11 Bullseye - glibc 2.31)...${NC}"
            ;;
        "alpine")
            dockerfile="Dockerfile.alpine"
            echo -e "${BLUE}Building with Docker (Alpine Linux - musl libc)...${NC}"
            ;;
        *)
            echo -e "${RED}Unknown base image: $base_image${NC}"
            echo "Available: ubuntu18, debian11, alpine"
            exit 1
            ;;
    esac
    
    # Build the image
    docker build -f "$dockerfile" -t "$image_name-$base_image" \
        --build-arg VERSION="$VERSION" \
        --build-arg GIT_COMMIT="$COMMIT" \
        --build-arg BUILD_DATE="$DATE" \
        .
    
    # Extract binary
    CONTAINER_ID=$(docker create "$image_name-$base_image")
    mkdir -p "$BUILD_DIR"
    docker cp "$CONTAINER_ID":/syschecks "$BUILD_DIR/syschecks-$base_image"
    docker rm "$CONTAINER_ID"
    
    # Create a symlink to the main binary name
    ln -sf "syschecks-$base_image" "$BUILD_DIR/syschecks"
    
    echo -e "${GREEN}Docker build complete! Binary: $BUILD_DIR/syschecks-$base_image${NC}"
}

build_native() {
    echo -e "${BLUE}Building natively...${NC}"
    mkdir -p "$BUILD_DIR"
    go build -ldflags="$LDFLAGS" -o "$BUILD_DIR/syschecks" .
    echo -e "${GREEN}Native build complete!${NC}"
}

build_static() {
    echo -e "${BLUE}Building static binary...${NC}"
    mkdir -p "$BUILD_DIR"
    CGO_ENABLED=0 go build -ldflags="$LDFLAGS -w -s -extldflags '-static'" -o "$BUILD_DIR/syschecks" .
    echo -e "${GREEN}Static build complete!${NC}"
}

build_cross() {
    echo -e "${BLUE}Cross-compiling for multiple architectures...${NC}"
    mkdir -p "$BUILD_DIR"
    
    # Define target platforms
    platforms=(
        "linux/amd64"
        "linux/arm64"
        "linux/386"
        "linux/arm"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )
    
    for platform in "${platforms[@]}"; do
        IFS='/' read -r GOOS GOARCH <<< "$platform"
        output_name="syschecks-$GOOS-$GOARCH"
        
        if [ "$GOOS" = "windows" ]; then
            output_name+='.exe'
        fi
        
        echo -e "${YELLOW}Building for $GOOS/$GOARCH...${NC}"
        env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
            go build -ldflags="$LDFLAGS -w -s" -o "$BUILD_DIR/$output_name" .
    done
    
    echo -e "${GREEN}Cross-compilation complete!${NC}"
}

show_info() {
    echo ""
    echo -e "${GREEN}Build Information:${NC}"
    echo "Build directory: $BUILD_DIR"
    
    if [ -f "$BUILD_DIR/syschecks" ]; then
        echo ""
        echo -e "${BLUE}Binary info:${NC}"
        file "$BUILD_DIR/syschecks"
        
        echo ""
        echo -e "${BLUE}Binary size:${NC}"
        ls -lh "$BUILD_DIR/syschecks"
        
        echo ""
        echo -e "${BLUE}Testing binary:${NC}"
        "$BUILD_DIR/syschecks" version || echo -e "${RED}Failed to run version command${NC}"
        
        echo ""
        echo -e "${BLUE}Compatibility notes:${NC}"
        echo "This binary should work on:"
        echo "  - Debian 9+"
        echo "  - Ubuntu 18.04+"
        echo "  - CentOS 7+/RHEL 7+"
        echo "  - Alpine Linux 3.10+"
        echo "  - Most other modern Linux distributions"
    fi
    
    if ls "$BUILD_DIR"/syschecks-* 1> /dev/null 2>&1; then
        echo ""
        echo -e "${BLUE}Cross-compiled binaries:${NC}"
        ls -la "$BUILD_DIR"/syschecks-*
    fi
}

# Main logic
case "${1:-docker}" in
    "docker")
        build_docker "${2:-ubuntu18}"
        ;;
    "native")
        build_native
        ;;
    "static")
        build_static
        ;;
    "cross")
        build_cross
        ;;
    "-h"|"--help"|"help")
        print_usage
        exit 0
        ;;
    *)
        echo -e "${RED}Unknown build method: $1${NC}"
        print_usage
        exit 1
        ;;
esac

show_info
