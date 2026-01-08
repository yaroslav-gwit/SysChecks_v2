# Docker Build Files

This directory contains all Docker-related files for building SysChecks with different base images.

## Available Dockerfiles

### Dockerfile (Default - Ubuntu 18.04 LTS)
- **Base:** Ubuntu 18.04 LTS
- **glibc:** 2.27
- **Best for:** Maximum compatibility with older Linux systems
- **Usage:** `./docker-build.sh` or `./build-advanced.sh docker ubuntu18`

**Compatibility:**
- Ubuntu 16.04+
- Debian 9+
- CentOS 7+/RHEL 7+
- Most modern Linux distributions

### Dockerfile.debian11
- **Base:** Debian 11 Bullseye
- **glibc:** 2.31
- **Best for:** Current stable systems
- **Usage:** `./build-advanced.sh docker debian11`

**Compatibility:**
- Debian 11+
- Ubuntu 20.04+
- More recent distributions

### Dockerfile.alpine
- **Base:** Alpine Linux
- **libc:** musl (not glibc)
- **Best for:** Smallest possible binaries, containerized environments
- **Usage:** `./build-advanced.sh docker alpine`

**Note:** Alpine builds use musl libc and may have different behavior than glibc-based systems.

### Dockerfile.multi
- **Type:** Multi-stage build
- **Targets:** 
  - `runtime` - Production-ready minimal image
  - `debug` - Development image with debugging tools
- **Usage:** `docker build -f docker/Dockerfile.multi --target runtime -t syschecks:runtime .`

## Building

### Quick Build (Default Ubuntu 18.04)

```bash
./docker-build.sh
```

### Advanced Build with Specific Base

```bash
# Ubuntu 18.04 (maximum compatibility)
./build-advanced.sh docker ubuntu18

# Debian 11 (current stable)
./build-advanced.sh docker debian11

# Alpine (smallest size)
./build-advanced.sh docker alpine
```

### Multi-stage Build

```bash
# Runtime image
docker build -f docker/Dockerfile.multi --target runtime -t syschecks:runtime .

# Debug image
docker build -f docker/Dockerfile.multi --target debug -t syschecks:debug .
```

## Documentation

- **DOCKER_BUILD.md** - Detailed Docker build documentation
- **DOCKER_BASE_COMPARISON.md** - Comparison of different base images

## Output

All Docker builds extract the compiled binary to `./bin/syschecks` in the project root.

## Why Multiple Dockerfiles?

Different base images serve different purposes:

1. **Ubuntu 18.04** - Maximum backward compatibility with older systems
2. **Debian 11** - Balance between modern features and stability
3. **Alpine** - Minimal size for containerized deployments
4. **Multi** - Flexible multi-stage builds for different use cases

Choose the Dockerfile that best matches your deployment target environment.
