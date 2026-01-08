# Changelog

All notable changes to SysChecks will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Enhanced release automation script with dry-run mode, checksums, and changelog integration
- SHA256 checksum generation for all release binaries
- Multi-architecture builds (Linux amd64/arm64, macOS amd64/arm64)
- Semantic version validation in release script
- Interactive confirmation prompts with `--force` override
- Verbose output mode for release debugging

### Changed
- Improved release script with better error handling and logging
- Release notes now automatically extract from CHANGELOG.md

### Fixed
- None

---

## [1.0.0] - 2026-01-08

### Added
- **Kernel Management**
  - `syschecks kernel` - Check if kernel reboot is needed (JSON output)
  - `syschecks kernel cleanup` - Clean up old kernel packages with `--keep` flag
  - Support for both generic and OEM kernels on Ubuntu/Debian

- **Update Monitoring**
  - `syschecks updates` - Check for system and security updates
  - Intelligent caching with `/tmp/syscheck_updates.json`
  - Support for apt (Debian/Ubuntu), yum (CentOS 7), and dnf (RHEL 8+, AlmaLinux, Rocky)
  - `--cache-create` and `--cache-use` flags for cache control

- **Automated Updates**
  - `syschecks apply-updates` - Apply security or system updates
  - Package lock file support (`/opt/syschecks/package.lock.json`)
  - `--ignore-lock-file` flag to bypass package locking
  - 10-minute timeout per package installation

- **Cron Job Management**
  - `syschecks cron init` - Set up update cache refresh (every 12 hours)
  - `syschecks cron updates --security` - Enable automatic security updates
  - `syschecks cron updates --system` - Enable automatic system updates
  - Randomized execution times to prevent thundering herd

- **Zabbix Integration**
  - `syschecks zabbix init` - Configure Zabbix agent UserParameter
  - JSON output for all monitoring commands
  - Compatible with Zabbix agent 1.x and 2.x

- **SSH Login Banner**
  - `syschecks banner` - Display system information on login
  - Shows OS, hostname, IPs, uptime, CPU, RAM, kernel status, updates
  - `--no-emojies` flag for minimal terminal support

- **System Information**
  - `syschecks sysinfo` - Display IP addresses (JSON)
  - `syschecks version` - Show version with `-v` for verbose output
  - Build metadata (commit, date, Go version) embedded at compile time

- **Build System**
  - Docker-based builds for maximum glibc compatibility
  - Cross-compilation for Linux, macOS, and Windows
  - Alpine Linux static builds (musl libc)
  - Automated release script with GitHub CLI integration

- **Installation**
  - One-line auto-install script
  - Bash completion support
  - Automatic cron job setup during installation
  - Configuration preservation on updates

### Supported Distributions
- Ubuntu 18.04+ (apt)
- Debian 9+ (apt)
- Pop!_OS (apt)
- CentOS 7 (yum)
- AlmaLinux 8+ (dnf)
- Rocky Linux 8+ (dnf)
- Oracle Linux 8+ (dnf)
- RHEL 8+ (dnf)

---

## Version History Format

Each release section follows this format:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- New features

### Changed
- Changes in existing functionality

### Deprecated
- Soon-to-be removed features

### Removed
- Removed features

### Fixed
- Bug fixes

### Security
- Security fixes
```

---

[Unreleased]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/yaroslav-gwit/SysChecks_v2/releases/tag/v1.0.0
