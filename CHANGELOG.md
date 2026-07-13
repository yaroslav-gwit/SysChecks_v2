# Changelog

All notable changes to SysChecks will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- None

### Changed
- None

### Fixed
- None

---

## [1.2.0] - 2026-07-13

### Added
- Login-banner warnings for writable disk filesystems with less than 10% space available (90% full); healthy filesystems remain silent.
- Installed-kernel count in both the banner and additive `installed_kernel_count` kernel JSON field, with an amber cleanup warning above six kernels.
- Logged-in user and session counts in the login banner.
- Current version and self-update cron status in the banner's top-right edge.
- Automatic OS-update policy in the banner, distinguishing full-system, security-only, conflicting, and fully disabled schedules; disabled/conflicting states are red.
- `syschecks kernel cleanup --execute` for guarded, non-shell package removal and `syschecks cron kernels [--keep N|--disable]` for weekly cleanup.
- Symmetric `--disable` handling for cache refresh and automatic update cron jobs.
- `syschecks cron` and `syschecks cron status` dashboard with live job state, server-time schedules, management commands, legacy detection, and conflict highlighting.
- `syschecks banner --all` debugging view that restores healthy and normally suppressed user, disk, kernel, update-policy, update-count, and cache details.
- Unit coverage for disk mount parsing and filtering, banner header/status rendering, `who`/`last` parsing, and kernel-retention safety.

### Changed
- Kernel retention now always protects at least the running kernel and a recent fallback.
- Banner health sections are exception-only: single-user state, healthy kernel/count, enabled OS-update policy, and zero update counts remain hidden.
- Enabling security-only or full-system update cron now removes the contradictory mode with a visible warning; contradictory flags in one invocation are rejected.
- Cron reconciliation also removes duplicate legacy `automatic_*` and `/etc/cron.d/syschecks` jobs left by pre-v2 installations.
- The human-readable `userinfo` table is compact enough for standard SSH terminals; full account/session detail remains available through JSON.
- Banner rendering no longer requires stdout to be a TTY.

### Fixed
- Last-login hosts and current session sources are parsed consistently from real `last -w -F` and `who` output.
- Banner and kernel checks now fall back safely on systems that do not expose `/boot/vmlinuz-*` files.

---

## [1.1.0] - 2026-07-01

### Added
- `syschecks self-update` — updates the binary in place from the latest GitHub release. Verifies the published SHA-256 checksum, replaces the running executable atomically (safe even while in use), and preserves file mode and ownership. Supports `--check` (report only) and `--force` (reinstall the same version). Honors an optional `GITHUB_TOKEN` for higher API rate limits.
- `syschecks cron autoupdate` — schedules a daily cron job that runs `self-update`; `--disable` removes it.
- Tests for semantic version comparison used by the self-updater.

### Fixed
- DNF/YUM update checks no longer parse package-manager warnings as packages. Queries now read stdout only, so messages such as `Repository <name> is listed more than once in the configuration` and `No security updates needed, but N update available` (emitted on stderr) can no longer inflate `security_updates`/`security_updates_list`. Reproduced and fixed on Rocky Linux 10.

---

## [1.0.2] - 2026-06-25

### Added
- Package-manager abstraction for apt, dnf, and yum backends.
- Broader distro detection using `/etc/os-release` `ID` and `ID_LIKE`, plus installed package-manager binaries.
- Support notes for additional apt and dnf/yum-compatible distributions.
- Real `syschecks userinfo` output with a pretty table by default.
- `syschecks userinfo --json`, `--json-pretty`, and `--all` flags.
- User inventory fields for active sessions, password lock state, last login time, login source, UID/GID, shell, home, and full name.
- End-to-end release validation runbook in `AgentDocs/E2E_TESTING.md`.
- Tests for package-manager detection, update parsers, kernel package matching, OS pretty-name fallback, and userinfo classification.

### Changed
- APT update checks now use the stable `apt-get` scripting interface instead of `apt`.
- DNF full-update checks now prefer `dnf repoquery --upgrades --qf` for machine-readable package data.
- DNF/YUM security checks now use advisory metadata from `updateinfo`.
- Package command execution now forces stable noninteractive environment settings.
- `apply-updates` now routes package operations through the selected package-manager backend.
- Kernel cleanup now uses package-manager backends for cleanup command generation.
- RPM kernel cleanup now discovers installed kernel packages dynamically from RPM ownership and installed package inventory instead of hard-coded package names.
- Kernel version comparison and cleanup now keep exact kernel release strings.
- Banner OS display-name detection now uses layered release metadata inspired by neofetch-style fallbacks.
- Documentation now reflects expanded distro support and release validation workflows.

### Fixed
- Security update counts no longer over-count multiple advisories for the same package.
- Empty update lists now serialize as `[]` instead of `null`.
- DNF `makecache` handling treats update-related exit codes correctly.
- Banner no longer falls back to the old "could not pick up OS name" message when common release metadata is available.

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

[Unreleased]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.0.1...v1.0.2
[1.0.0]: https://github.com/yaroslav-gwit/SysChecks_v2/releases/tag/v1.0.0
