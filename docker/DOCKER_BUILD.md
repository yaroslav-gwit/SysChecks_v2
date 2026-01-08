# Docker Build Guide for SysChecks

## Overview

This project includes Docker-based building to ensure maximum compatibility across different Linux distributions. By building on Debian 10, we can create binaries that work on older systems while maintaining modern Go features.

## Why Debian 10?

Building on Debian 10 (Buster) with glibc 2.28 ensures compatibility with:
- Debian 9+ (Stretch and newer)
- Ubuntu 18.04+ (Bionic and newer)
- CentOS 7+/RHEL 7+
- Alpine Linux 3.10+
- Most other modern Linux distributions

If you build on a newer system like Ubuntu 24.04, the binary will have dependencies on newer glibc versions that won't be available on older systems.

## Build Methods

### 1. Docker Build (Recommended for Distribution)

```bash
# Simple Docker build
./docker-build.sh

# Or manually:
docker build -t syschecks-builder .
CONTAINER_ID=$(docker create syschecks-builder)
docker cp "$CONTAINER_ID":/syschecks ./bin/syschecks
docker rm "$CONTAINER_ID"
```

### 2. Advanced Build Script

```bash
# Docker build with detailed output
./build-advanced.sh docker

# Native build (fastest for development)
./build-advanced.sh native

# Static binary build (good compromise)
./build-advanced.sh static

# Cross-compile for multiple platforms
./build-advanced.sh cross
```

### 3. Multi-stage Docker Build

```bash
# Build minimal runtime image
docker build -f Dockerfile.multi --target runtime -t syschecks:runtime .

# Build debug image (with shell)
docker build -f Dockerfile.multi --target debug -t syschecks:debug .
```

## Docker Files

- `Dockerfile` - Main build file using Debian 10
- `Dockerfile.multi` - Multi-stage build with runtime and debug targets
- `.dockerignore` - Optimizes build context
- `docker-build.sh` - Simple build and extract script
- `build-advanced.sh` - Comprehensive build script with multiple options

## Binary Compatibility

The Docker-built binary includes these optimizations:
- `CGO_ENABLED=0` - Fully static binary with no C dependencies
- `-ldflags='-w -s'` - Strips debug info for smaller size
- `-extldflags "-static"` - Ensures static linking

## Testing Compatibility

After building, test the binary on your target systems:

```bash
# Check binary type
file ./bin/syschecks

# Test on target system
./bin/syschecks version
./bin/syschecks kernel --help
./bin/syschecks cleanup --help
```

## Size Comparison

- Native build: ~8-12 MB (depends on host system)
- Docker static build: ~6-8 MB (optimized and stripped)
- Cross-compiled: ~6-8 MB per platform

## Development Workflow

1. **Development**: Use `./build-advanced.sh native` for fast iterations
2. **Testing**: Use `./build-advanced.sh static` for local compatibility testing
3. **Distribution**: Use `./build-advanced.sh docker` for maximum compatibility
4. **Release**: Use `./build-advanced.sh cross` for multi-platform releases
