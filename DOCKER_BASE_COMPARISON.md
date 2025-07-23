# Docker Base Image Comparison

Since Debian 10 is EOL, here are the best alternatives for building portable Go binaries:

## Recommended Options

### 1. Ubuntu 18.04 LTS (Recommended)
- **Dockerfile**: `Dockerfile` (default)
- **glibc**: 2.27
- **Support**: Until April 2028 (ESM)
- **Compatibility**: Ubuntu 16.04+, Debian 9+, CentOS 7+
- **Use case**: Best balance of compatibility and support

```bash
./build-advanced.sh docker ubuntu18
```

### 2. Alpine Linux (Smallest binaries)
- **Dockerfile**: `Dockerfile.alpine`
- **libc**: musl (not glibc)
- **Size**: Smallest possible binaries
- **Compatibility**: Excellent (static binaries work everywhere)
- **Use case**: When you want the smallest possible binaries

```bash
./build-advanced.sh docker alpine
```

### 3. Debian 11 Bullseye (Most recent)
- **Dockerfile**: `Dockerfile.debian11`
- **glibc**: 2.31
- **Support**: Until ~2026
- **Compatibility**: Debian 10+, Ubuntu 20.04+
- **Use case**: When you need newer base system

```bash
./build-advanced.sh docker debian11
```

## Compatibility Matrix

| Base Image | glibc | Min Ubuntu | Min Debian | Min CentOS | Binary Size |
|------------|-------|------------|------------|------------|-------------|
| Ubuntu 18.04 | 2.27 | 16.04 | 9 | 7 | ~6-8 MB |
| Alpine 3.18 | musl | Any | Any | Any | ~5-7 MB |
| Debian 11 | 2.31 | 20.04 | 10 | 8 | ~6-8 MB |

## Quick Test

After building, test compatibility:

```bash
# Check what the binary was built against
ldd ./bin/syschecks-ubuntu18  # Should show "not a dynamic executable"
file ./bin/syschecks-ubuntu18

# Test on target systems
./bin/syschecks-ubuntu18 version
```

## Recommendation

For maximum compatibility: **Ubuntu 18.04**  
For minimum size: **Alpine Linux**  
For newest features: **Debian 11**
