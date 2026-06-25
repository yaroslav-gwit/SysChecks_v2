# SysChecks v2

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Linux](https://img.shields.io/badge/OS-Linux-FCC624?logo=linux&logoColor=black)](https://www.linux.org/)

A powerful, production-ready CLI tool for Linux system administration and monitoring.
Built in Go for speed and reliability, SysChecks extends Zabbix functionality with kernel management, update monitoring, automated maintenance, and beautiful system banners.

## ✨ Features

- 🔍 **Kernel Management** - Check reboot requirements and clean up old kernels
- 📦 **Update Monitoring** - Track system and security updates with caching
- 🤖 **Automated Updates** - Schedule automatic security/system updates via cron
- 📊 **Zabbix Integration** - Native UserParameter support for monitoring
- 🎨 **SSH Login Banner** - Beautiful system info display on login
- ⚡ **Performance** - Fast execution with intelligent caching
- 🔒 **Package Locking** - Prevent specific packages from auto-updating
- 🐧 **Multi-Distro** - Supports Debian/Ubuntu, RHEL/CentOS/AlmaLinux/Rocky

## 🚀 Quick Start

### One-Line Installation

```bash
curl -sSL https://raw.githubusercontent.com/yaroslav-gwit/SysChecks_v2/main/auto-install.sh | sudo bash
```

That's it! The script will:
- Download the latest release binary
- Install to `/opt/syschecks/` and link to `/usr/bin/`
- Set up bash completion (if available)
- Configure cron jobs for update caching and automatic security updates
- Preserve your configuration on updates

### Manual Installation

```bash
# Download the binary
VERSION="1.0.0"  # Replace with desired version
wget https://github.com/yaroslav-gwit/SysChecks_v2/releases/download/v${VERSION}/syschecks-linux-amd64

# Install
sudo mkdir -p /opt/syschecks
sudo mv syschecks-linux-amd64 /opt/syschecks/syschecks
sudo chmod +x /opt/syschecks/syschecks
sudo ln -s /opt/syschecks/syschecks /usr/bin/syschecks

# Enable bash completion
sudo syschecks completion bash > /etc/bash_completion.d/syschecks
```

## 📖 Usage

### Check Kernel Status

```bash
# Check if kernel reboot is needed
syschecks kernel

# Pretty-printed JSON output
syschecks kernel --json-pretty

# Sample output:
{
   "kernel_needs_reboot": true,
   "running_kernel": "5.15.0-91",
   "latest_installed_kernel": "5.15.0-92",
   "active_kernels": ["5.15.0-91", "5.15.0-92"]
}
```

### Clean Up Old Kernels

```bash
# Keep 4 most recent kernels (default)
sudo syschecks kernel cleanup

# Keep only 2 kernels
sudo syschecks kernel cleanup --keep 2
```

### Check for Updates

```bash
# Check for updates (uses cache by default)
syschecks updates

# Force fresh check and create new cache
sudo syschecks updates --cache-create

# Pretty output
syschecks updates --json-pretty
```

### Apply Updates

```bash
# Apply security updates only (default)
sudo syschecks apply-updates

# Apply all system updates
sudo syschecks apply-updates --system

# Ignore package lock file
sudo syschecks apply-updates --ignore-lock-file
```

### Display System Banner

```bash
# Show system information banner
syschecks banner

# Without emojis
syschecks banner --no-emojies
```

To show banner on SSH login:

```bash
echo '([ -z "$PS1" ] && true) || syschecks banner' | sudo tee /etc/profile.d/syschecks_banner.sh
sudo chmod +x /etc/profile.d/syschecks_banner.sh
```

### Cron Job Management

```bash
# Set up update cache refresh (runs every 12 hours)
sudo syschecks cron init

# Enable automatic security updates (runs daily at 4:15 AM)
sudo syschecks cron updates --security

# Enable automatic system updates
sudo syschecks cron updates --system
```

### Zabbix Integration

```bash
# Initialize Zabbix agent integration
sudo syschecks zabbix init
```

This adds a UserParameter to your Zabbix agent configuration:

```ini
UserParameter=syschecks[*],syschecks $1
```

**Usage in Zabbix:**

- Item key: `syschecks[kernel]` - Kernel status
- Item key: `syschecks[updates]` - Update status
- Item key: `syschecks[sysinfo]` - System IP addresses

## 🔧 Configuration

### Package Lock File

Prevent specific packages from being updated automatically:

**Location:** `/opt/syschecks/package.lock.json`

```json
[
  "docker-ce",
  "docker-ce-cli",
  "nvidia-driver",
  "linux-headers"
]
```

Locked packages will be skipped during automatic updates. Use `--ignore-lock-file` flag to override.

### Cache Files

- `/tmp/syscheck_updates.json` - Update check cache (refreshed every 12 hours)

### Cron Jobs

- `/etc/cron.d/syschecks_cache` - Update cache refresh
- `/etc/cron.d/syschecks_updates_security` - Automatic security updates
- `/etc/cron.d/syschecks_updates_system` - Automatic system updates

### Logs

- `/var/log/syschecks_updates.log` - Automatic update log

## 📊 Supported Linux Distributions

| Distribution | Package Manager | Status |
|--------------|----------------|--------|
| Ubuntu 18.04+ | apt | ✅ Tested |
| Debian 9+ | apt | ✅ Tested |
| Pop!_OS | apt | ✅ Tested |
| Linux Mint / Elementary / Kali / Raspberry Pi OS / Zorin / KDE neon | apt | ✅ Supported |
| CentOS 7 | yum | ✅ Tested |
| AlmaLinux 8+ | dnf | ✅ Tested |
| Rocky Linux 8+ | dnf | ✅ Tested |
| RHEL 8+ | dnf | ✅ Tested |
| Oracle Linux 8+ | dnf | ✅ Tested |
| Fedora / Amazon Linux / openEuler | dnf/yum | ✅ Supported |

Package manager detection also uses `ID_LIKE` from `/etc/os-release` and falls back to the available `apt-get`, `dnf`, or `yum` binary for compatible derivatives.

## 🔍 Command Reference

```
syschecks
├── kernel [--json-pretty]        # Check kernel reboot status
│   └── cleanup [--keep N]        # Clean up old kernels
├── updates [flags]                # Check for updates
│   ├── --json-pretty              # Pretty JSON output
│   ├── --cache-create             # Create new cache
│   └── --cache-use                # Use existing cache (default)
├── apply-updates [flags]          # Apply updates
│   ├── --system, -s               # Apply all updates (not just security)
│   └── --ignore-lock-file, -i     # Ignore package lock
├── banner [--no-emojies, -n]      # Display system banner
├── cron                           # Cron job management
│   ├── init                       # Set up cache cron
│   └── updates [--security|--system]  # Enable auto-updates
├── zabbix                         # Zabbix integration
│   └── init                       # Initialize Zabbix support
├── sysinfo                        # System IP addresses (JSON)
├── userinfo [--json|--json-pretty] [--all]
│                                   # Real users, login state, password status
└── version [--verbose, -v]        # Show version info
```

## 🐛 Troubleshooting

### Command not found

Ensure `/usr/bin` is in your PATH:

```bash
echo $PATH
which syschecks
```

### Permission denied

Most commands work without root, but some require it:

```bash
sudo syschecks updates --cache-create
sudo syschecks apply-updates
sudo syschecks kernel cleanup
```

### Cache is stale

Refresh the cache manually:

```bash
sudo syschecks updates --cache-create
```

### Zabbix not picking up metrics

1. Verify UserParameter was added:
   ```bash
   grep syschecks /etc/zabbix/zabbix_agentd.conf
   ```

2. Test from command line:
   ```bash
   syschecks kernel
   ```

3. Test via zabbix_get:
   ```bash
   zabbix_get -s 127.0.0.1 -k "syschecks[kernel]"
   ```

### Package lock not working

Verify the file exists and has correct permissions:

```bash
ls -la /opt/syschecks/package.lock.json
cat /opt/syschecks/package.lock.json
```

## 🏗️ Building from Source

### Prerequisites

- Go 1.21 or later
- Git

### Build

```bash
# Clone the repository
git clone https://github.com/yaroslav-gwit/SysChecks_v2.git
cd SysChecks_v2

# Standard build
./build.sh

# The binary will be in ./bin/syschecks
./bin/syschecks version
```

### Build Options

```bash
# Cross-compilation
./build-advanced.sh

# Docker-based build (for maximum compatibility)
./docker-build.sh

# Manual build with version info
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')

go build -ldflags="-X 'syschecks/cmd.Version=$VERSION' \
  -X 'syschecks/cmd.GitCommit=$COMMIT' \
  -X 'syschecks/cmd.BuildDate=$DATE'" \
  -o ./bin/syschecks
```

## 📚 Documentation

- [Changelog](CHANGELOG.md) - Version history and release notes
- [Releases Guide](RELEASES.md) - How to create releases
- [Architecture Overview](AgentDocs/ARCHITECTURE.md) - System design and data flow
- [Command Reference](AgentDocs/COMMANDS.md) - Detailed command documentation
- [Development Guide](AgentDocs/DEVELOPMENT.md) - Contributing and development setup
- [E2E Test Plan](AgentDocs/E2E_TESTING.md) - Release validation checklist
- [Zabbix Integration](AgentDocs/ZABBIX_INTEGRATION.md) - Zabbix setup and templates
- [Project Constitution](AgentDocs/CONSTITUTION.md) - Coding standards and best practices

## 🤝 Contributing

Contributions are welcome! Please read our [Development Guide](AgentDocs/DEVELOPMENT.md) and [Constitution](AgentDocs/CONSTITUTION.md) before submitting PRs.

### Quick Contribution Guide

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Follow the coding standards in [CONSTITUTION.md](AgentDocs/CONSTITUTION.md)
4. Test your changes thoroughly
5. Commit with clear messages: `feat: add new feature`
6. Push and create a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Inspired by the need for better system monitoring tools
- Thanks to all contributors and users

## 📧 Support

- **Issues:** [GitHub Issues](https://github.com/yaroslav-gwit/SysChecks_v2/issues)
- **Discussions:** [GitHub Discussions](https://github.com/yaroslav-gwit/SysChecks_v2/discussions)

---

**Made with ❤️ for Linux sysadmins**
