# GitHub Releases Guide

This guide explains how to programmatically publish releases for SysChecks to GitHub.

## 🚀 Quick Start

The easiest way to create a release:

```bash
# Install GitHub CLI
# Ubuntu/Debian: apt install gh
# macOS: brew install gh
# Or download from: https://cli.github.com/

# Authenticate
gh auth login

# Create release (builds binaries automatically)
./release.sh 1.0.0

# Dry run first (recommended)
./release.sh -n 1.0.0
```

## 📋 Available Methods

### 1. GitHub CLI Script (Recommended)

**File**: `release.sh`
**Best for**: Manual releases, full automation

```bash
# Create regular release
./release.sh 1.0.0

# Create prerelease
./release.sh 1.0.0-beta.1 prerelease

# Create draft
./release.sh 1.0.0 draft

# Dry run (simulate without making changes)
./release.sh -n 1.0.0

# Skip confirmation prompts
./release.sh -f 1.0.0

# Use existing binaries (skip build)
./release.sh -s 1.0.0

# Verbose output
./release.sh -v 1.0.0
```

**Features**:

- ✅ Semantic version validation
- ✅ Automatic changelog extraction from CHANGELOG.md
- ✅ Multi-architecture builds (Linux amd64/arm64, macOS amd64/arm64)
- ✅ SHA256 checksum generation
- ✅ Docker-based portable Linux builds
- ✅ Dry-run mode for testing
- ✅ Interactive confirmations (or `--force` to skip)
- ✅ Existing release/tag cleanup
- ✅ Colored output with progress logging

**Prerequisites**:

- GitHub CLI installed and authenticated
- Docker (for portable Linux binaries, optional)
- Go 1.21+ (for cross-compilation)

### 2. GitHub Actions (Automated)

**File**: `.github/workflows/release.yml`
**Best for**: Automated releases on git tags

```bash
# Create and push a tag to trigger release
git tag v1.0.0
git push origin v1.0.0
```

**Features**:

- Automatically builds cross-platform binaries
- Creates Docker variants (Ubuntu 18.04, Alpine)
- Generates checksums
- No manual intervention required

### 3. Command Line Options Reference

| Option | Long Form | Description |
|--------|-----------|-------------|
| `-h` | `--help` | Show help message |
| `-n` | `--dry-run` | Simulate release without changes |
| `-f` | `--force` | Skip confirmation prompts |
| `-s` | `--skip-build` | Use existing binaries in ./bin/ |
| `-t` | `--skip-tests` | Skip running tests before release |
| `-v` | `--verbose` | Enable verbose output |

## 🔧 Setup Instructions

### GitHub CLI Setup

```bash
# Install GitHub CLI
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update && sudo apt install gh

# Authenticate
gh auth login
```

### GitHub Token Setup

1. Go to https://github.com/settings/tokens
2. Click "Generate new token (classic)"
3. Select scopes: `repo` (full repository access)
4. Copy the token and set it as environment variable:

```bash
   export GITHUB_TOKEN="ghp_your_token_here"
```

### GitHub Actions Setup

The workflow is already configured in `.github/workflows/release.yml`. It will automatically trigger when you push a tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 📦 Binary Variants

Each release includes multiple binary variants:

| Binary | Platform | Compatibility | Use Case |
|--------|----------|---------------|----------|
| `syschecks-linux-amd64` | Linux x64 | Ubuntu 16.04+ | Standard Linux servers |
| `syschecks-linux-arm64` | Linux ARM64 | Most ARM64 systems | ARM servers (AWS Graviton, etc.) |
| `checksums-sha256.txt` | N/A | N/A | Verification checksums |

> **Note:** SysChecks is a Linux-only tool. macOS and Windows are not supported.

## 🔄 Release Workflow

### Manual Release Process

1. **Update Changelog**: Add entry in `CHANGELOG.md` for the new version
2. **Dry Run**: Test with `./release.sh -n X.Y.Z`
3. **Review**: Check the summary output
4. **Release**: Run `./release.sh X.Y.Z`
5. **Verify**: Check the release on GitHub
6. **Announce**: Update documentation, notify users

### Automated Release Process

1. **Test**: Ensure all tests pass
2. **Tag**: Create and push a git tag

   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **Wait**: GitHub Actions automatically builds and publishes
4. **Verify**: Check the release was created successfully

## 📝 Release Notes

The release script automatically generates release notes with:

- A friendly intro paragraph (see below)
- Version-specific changes from CHANGELOG.md (if available)
- Feature highlights (fallback if no changelog entry)
- Binary download instructions for all platforms
- Checksum verification commands
- Quick start examples
- Auto-install script link

### Friendly intros (for LLM agents cutting a release)

Every release body opens with a short, warm intro. **Do not ship the generic
fallback line** — write a human, 2-4 sentence summary of *this* release in plain
language: what users get, why it matters, and any headline features or important
fixes. Upbeat but honest; markdown and emoji are welcome.

Provide it dynamically, in priority order:

1. **`RELEASE_INTRO` env var (preferred)** — set it right before running the script:

   ```bash
   export RELEASE_INTRO="In this release we're excited to make SysChecks self-maintaining, plus a fix for noisy update reports on some RHEL hosts. 🚀"
   ./release.sh X.Y.Z
   ```

2. **`release-notes/vX.Y.Z.md` file** — commit the intro text for the version:

   ```bash
   printf '%s\n' "In this release ..." > release-notes/vX.Y.Z.md
   ./release.sh X.Y.Z
   ```

If neither is provided, a generic fallback is used (fine for automation, but a
tailored intro is strongly preferred for human-facing releases). The intro sits
between the title and the changelog sections; keep it to a paragraph.

### Changelog Integration

Create entries in `CHANGELOG.md` following this format:

```markdown
## [1.2.0] - 2026-01-15

### Added
- New feature X
- Support for Y

### Fixed
- Bug in Z
```

The release script will automatically extract this section for release notes.

## 🛠 User Installation

Users can install your releases easily:

```bash
# Auto-install latest version
curl -fsSL https://raw.githubusercontent.com/yaroslav-gwit/SysChecks_v2/main/auto-install.sh | bash

# Install specific version
curl -fsSL https://raw.githubusercontent.com/yaroslav-gwit/SysChecks_v2/main/auto-install.sh | bash -s -- --version 1.0.0

# Install to custom directory
curl -fsSL https://raw.githubusercontent.com/yaroslav-gwit/SysChecks_v2/main/auto-install.sh | bash -s -- --dir ~/.local/bin
```

## 🔍 Troubleshooting

### Common Issues

**"gh: command not found"**

- Install GitHub CLI: https://cli.github.com/

**"Not authenticated with GitHub CLI"**

- Run: `gh auth login`

**"Invalid version format"**

- Use semantic versioning: `1.0.0`, `1.0.0-beta.1`, `2.0.0-rc.1`

**"Tag already exists"**

- Use `--force` to delete and recreate, or choose a different version

**"Docker build failed"**

- Ensure Docker is installed and running: `docker info`
- The script will fall back to native builds if Docker fails

**"Permission denied" during Docker build**

- The script uses `sudo` for Docker builds
- Alternatively, add your user to the docker group

### Debugging

```bash
# Check GitHub CLI authentication
gh auth status

# Test dry run
./release.sh -n -v 1.0.0

# Verify local build works
./build-advanced.sh native

# Check existing releases
gh release list --repo yaroslav-gwit/SysChecks_v2

# Check existing tags
git tag -l
```

## 📊 Best Practices

1. **Update Changelog First**: Add entries to CHANGELOG.md before releasing
2. **Dry Run**: Always test with `-n` flag first
3. **Semantic Versioning**: Follow semver.org guidelines
4. **Clean Working Tree**: Commit all changes before releasing
5. **Test Locally**: Run `./build.sh` and test the binary before release
6. **Monitor Issues**: Watch for bug reports after releasing

## 🔗 Related Files

- [CHANGELOG.md](CHANGELOG.md) - Version history and release notes
- [build.sh](build.sh) - Simple build script
- [build-advanced.sh](build-advanced.sh) - Advanced build with Docker/cross-compilation
- [auto-install.sh](auto-install.sh) - User installation script
