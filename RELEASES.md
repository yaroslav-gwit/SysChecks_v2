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
```

## 📋 Available Methods

### 1. GitHub CLI Script (Recommended)

**File**: `release.sh`
**Best for**: Manual releases, simplicity

```bash
# Create regular release
./release.sh 1.0.0

# Create prerelease
./release.sh 1.0.0-beta prerelease

# Create draft
./release.sh 1.0.0 draft
```

**Prerequisites**:

- GitHub CLI installed and authenticated
- Docker (for building portable binaries)

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

### 3. REST API Script (Advanced)

**File**: `release-api.sh`
**Best for**: CI/CD integration, maximum control

```bash
# Set up token
export GITHUB_TOKEN="ghp_your_token_here"

# Create release
./release-api.sh 1.0.0

# Create prerelease
./release-api.sh 1.0.0-beta --prerelease

# Create draft
./release-api.sh 1.0.0 --draft

# Skip building (use existing binaries)
./release-api.sh 1.0.0 --skip-build
```

**Prerequisites**:

- GitHub Personal Access Token with `repo` scope
- `curl` and `jq` installed

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
| `syschecks-linux-amd64` | Linux x64 | Ubuntu 16.04+ | Standard Linux |
| `syschecks-linux-arm64` | Linux ARM64 | Most ARM64 systems | ARM servers |
| `syschecks-darwin-amd64` | macOS Intel | macOS 10.12+ | Intel Macs |
| `syschecks-darwin-arm64` | macOS Apple Silicon | macOS 11+ | M1/M2 Macs |
| `syschecks-windows-amd64.exe` | Windows x64 | Windows 7+ | Windows systems |
| `syschecks-ubuntu18` | Linux x64 | Ubuntu 16.04+ | Max compatibility |
| `syschecks-alpine` | Linux x64 | Any Linux | Minimal static |

## 🔄 Release Workflow

### Manual Release Process

1. **Test**: Ensure all tests pass
2. **Build**: Test build locally with `./build-advanced.sh cross`
3. **Version**: Decide on version number (semantic versioning)
4. **Release**: Run `./release.sh <version>`
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

## 📝 Release Notes Template

The scripts automatically generate release notes with:

- Feature highlights
- Binary download links
- Installation instructions
- Usage examples
- Checksum verification

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

**"curl: command not found"**

- Install curl: `sudo apt install curl`

**"jq: command not found"**

- Install jq: `sudo apt install jq`

**"Permission denied" during release**

- Check GitHub token permissions
- Ensure token has `repo` scope

**"Docker build failed"**

- Ensure Docker is installed and running
- Check Dockerfile syntax

### Debugging

```bash
# Test GitHub CLI authentication
gh auth status

# Test API access
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user

# Test local build
./build-advanced.sh docker ubuntu18

# Dry run release (API script)
./release-api.sh 1.0.0-test --draft
```

## 📊 Best Practices

1. **Version Numbers**: Use semantic versioning (1.0.0, 1.1.0, 2.0.0)
2. **Testing**: Always test builds before releasing
3. **Automation**: Use GitHub Actions for consistent releases
4. **Documentation**: Keep release notes informative
5. **Security**: Use environment variables for tokens
6. **Backup**: Keep release scripts in version control
