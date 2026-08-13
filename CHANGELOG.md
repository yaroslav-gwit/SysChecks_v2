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

## [1.3.3] - 2026-08-13

### Fixed
- Regular-user banners no longer report automatic OS updates and SysChecks self-update as
  disabled when a hardened host intentionally prevents traversal of `/etc/cron.d`. Privileged
  schedule changes and update refreshes now record a root-owned schedule snapshot in the
  existing status cache; live cron state remains authoritative whenever it is readable.
- Missing, unreadable, or untrusted update caches no longer receive a fabricated creation
  timestamp or turn zero-value update and repository fields into healthy checks. The status
  cache is written atomically as `0644` despite restrictive umasks and rejected unless it is
  a root-owned regular file that cannot be modified by group or other users.

---

## [1.3.2] - 2026-08-13

### Changed
- Alpine's unsupported security-only channel is no longer a default human-banner warning.
  It is shown only when a security-only scheduled job is actually present; update JSON
  continues to report the capability explicitly as `unsupported` with `null` values.

### Fixed
- Alpine kernel status now ignores unversioned boot flavor aliases such as `vmlinuz-virt`
  and discovers the exact installed kernel release from `/lib/modules`, restoring accurate
  latest-kernel and reboot-required checks.

---

## [1.3.1] - 2026-08-12

### Added
- Alpine Linux support through a native `apk` package-manager backend for update checks and full-system update application. Existing static release binaries continue to run on musl; no separate Alpine binary is required.
- Explicit `system_updates_status` and `security_updates_status` fields in update JSON. Alpine reports security-only data as `unsupported`, with the corresponding count, availability, and list fields set to `null` rather than a misleading zero/false/empty result.

### Changed
- Alpine scheduling requires the `cronie` package. `schedule enable` and its legacy aliases now fail loudly when only BusyBox `crond` is installed because BusyBox ignores `/etc/cron.d`.
- Alpine kernel cleanup is a no-op because `apk` normally upgrades kernel packages in place instead of retaining parallel package versions.
- `migrate --apply` also restores generated cron files and the update cache to mode `0644`, making status visible to regular-user login banners.

### Fixed
- `updates check` and `banner` now validate OS/package-manager support before taking the cached-read shortcut. Unsupported systems exit non-zero instead of reporting zero pending updates with exit code 0.
- Security-only update application and scheduling on Alpine now report that apk cannot answer the request from Alpine secdb data instead of treating the unknown result as zero.
- Cron and cache writers now apply `0644` after writing, so restrictive root umasks and pre-existing `0600` files no longer make regular-user banners report jobs/cache as missing while root sees them.

---

## [1.3.0] - 2026-07-24

### Added
- Resource-then-verb command structure: `updates check|apply|refresh`, `kernel status|cleanup`, `users list`, and `schedule list|enable|disable <job>`. Every previous spelling still works (see Deprecated).
- Global `-o, --output text|json|json-pretty` flag on every command, with shell completion on its values. Job names for `schedule enable|disable` and values for `--scope` complete too.
- `syschecks banner --output json`, reporting every banner check — including the healthy ones the human banner suppresses — so monitoring can tell "check passed" from "check missing". Absorbs everything `sysinfo` printed, under the same `ip_address_list` field name.
- `syschecks migrate` (report-only) and `syschecks migrate --apply`, which rewrite generated cron files and Zabbix `UserParameter` lines that reference pre-restructure command names. Exits non-zero while changes are outstanding, so monitoring can ask whether a host is fully migrated. Called automatically by `auto-install.sh` and `install-offline.sh`.
- `syschecks updates apply --dry-run`, listing what would be installed (including which packages `package.lock.json` would skip) without changing anything.
- Randomized start delay for unattended update runs (`--delay`, default 15m; `--no-delay` to disable), so guests sharing a virtualization host do not all update in the same minute. Interactive runs never wait.
- Broken-repository detection on the banner and in the update cache (`repository_issues`, `repository_issue_count`), covering missing GPG keys, HTTP 404, unresolvable hosts and TLS failures on apt, DNF4 and DNF5.
- `Enabled` column in `schedule list` with a status symbol (`✅ yes`, `🟡 yes` legacy, `🛑 yes` conflict, `❌ no`).

### Changed
- **BREAKING:** `syschecks kernel cleanup` now removes old kernel packages by default. Previously it only previewed them unless `--execute` was given. Use `syschecks kernel cleanup --dry-run` for the old preview behaviour. `--execute` is still accepted but deprecated and has no effect beyond the new default. Run interactively without `--yes`, the command now lists the packages and asks for confirmation; unattended runs (cron) proceed without prompting. This aligns kernel cleanup with every other mutating command, which now share a single `--dry-run` convention.
- `userinfo` renamed to `users`; the `Active` column is now `Logged in`, and the JSON fields `active`/`active_sessions`/`active_sources` are now `logged_in`/`login_sessions`/`login_sources`. "Active" was read as "the account is enabled", which is what the `Password` column actually reports.
- Generated cron jobs now use the new command spellings. Existing jobs are rewritten by `syschecks migrate --apply`.
- `schedule list` (formerly `cron status`) shows the `syschecks schedule ...` command to run for each job instead of the old `cron` subcommand.
- Tables size and pad by display width instead of byte length, so cells containing emoji or non-ASCII text no longer stagger the right-hand border.

### Deprecated
- `apply-updates` → `updates apply`; `--system`/`-s` → `--scope system`; `--ignore-lock-file`/`-i` → `--ignore-locks`.
- `updates --cache-create` → `updates refresh` (also spelled `updates cache refresh`); `updates --cache-use` → `updates check --refresh` for a live check.
- `--json` / `--json-pretty` on every command → `--output json` / `--output json-pretty`.
- `cron` → `schedule`; `cron status` → `schedule list`; `cron init|updates|autoupdate|kernels [--disable]` → `schedule enable|disable <job>`.
- `sysinfo` → `banner --output json`.
- `banner --no-emojies` → `--no-emojis`.
- All of the above keep working for at least one full release cycle.

### Fixed
- `updates --cache-create` wrote `cache_exists: false` and `cache_up_to_date: false` into the cache file it had just created, so anything reading the JSON directly saw a cache that claimed not to exist.
- Security update counts on DNF5 counted timestamps instead of packages: the parser took the last whitespace field, which is the `Issued` time in DNF5's five-column layout. On a Fedora 44 host this reported 10 security updates where there were 59, and that number is shown in the login banner.
- The `dnf repoquery` fast path returned a single malformed entry on DNF5, which suppressed the `check-update` fallback and under-reported system updates (1 instead of 174 on a Fedora 44 host). DNF5 emits the `--qf` format verbatim with no record separator, so the format string now carries its own newline.
- Broken repositories were reported nowhere on Fedora: DNF5 exits 0 and prints "Metadata cache created." even when a repository fails completely, writing its errors only to stderr. `apt-get update` likewise exits 0 when a repository host stops resolving.
- The kernel-cleanup banner line overflowed an 80-column banner, and its `⚠️` emoji (a variation-selector sequence) measured one column but rendered as two, shifting that line's right border.

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

[Unreleased]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.3.3...HEAD
[1.3.3]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.3.2...v1.3.3
[1.3.2]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/yaroslav-gwit/SysChecks_v2/compare/v1.0.1...v1.0.2
[1.0.0]: https://github.com/yaroslav-gwit/SysChecks_v2/releases/tag/v1.0.0
