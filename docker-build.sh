#!/bin/bash

# Build script for SysChecks using Docker for maximum compatibility

set -e

echo "Building SysChecks using Ubuntu 18.04 LTS for maximum compatibility..."

# Get version information from git
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')

# Build the Docker image
docker build -t syschecks-builder \
    --build-arg VERSION="$VERSION" \
    --build-arg GIT_COMMIT="$COMMIT" \
    --build-arg BUILD_DATE="$DATE" \
    .

# Create a temporary container to extract the binary
CONTAINER_ID=$(docker create syschecks-builder)

# Extract the binary
docker cp "$CONTAINER_ID":/syschecks ./bin/syschecks

# Clean up the temporary container
docker rm "$CONTAINER_ID"

echo "Build complete! Binary available at: ./bin/syschecks"
echo ""
echo "Testing the binary:"
./bin/syschecks version
echo ""
echo "Binary info:"
file ./bin/syschecks
echo ""
echo "Binary size:"
ls -lh ./bin/syschecks
echo ""
echo "The binary should now work on:"
echo "  - Ubuntu 16.04+"
echo "  - Debian 9+"
echo "  - CentOS 7+/RHEL 7+"
echo "  - Most other modern Linux distributions"
