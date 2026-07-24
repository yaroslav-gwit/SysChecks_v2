# SysChecks Command Reference

## Command Tree

Commands are `syschecks <resource> <verb>`. Every pre-v1.3.0 spelling still resolves as a
hidden, deprecated alias; `syschecks migrate` rewrites generated files that use them.

```
syschecks [-o|--output text|json|json-pretty]     # global, tab-completable
├── banner [--all] [--no-emojis, -n] [--disk-used-threshold PERCENT]
│                                 # Login banner; -o json reports every check
├── updates                       # Update resource
│   ├── check [--refresh|--cached]
│   │                             # Report available updates (cached by default)
│   ├── apply [--scope security|system] [--dry-run] [--ignore-locks]
│   │         [--delay DURATION] [--no-delay]
│   │                             # Install updates
│   ├── refresh                   # Refresh the update cache
│   └── cache refresh             # Same, longer spelling
├── kernel                        # Kernel resource
│   ├── status                    # Reboot-required check (JSON)
│   └── cleanup [--keep N] [--dry-run] [--yes, -y]
│                                 # REMOVES old kernels (--dry-run to preview)
├── users                         # User resource
│   └── list [--all]              # Users and who is logged in
├── schedule                      # Scheduled jobs (was: cron)
│   ├── list                      # Job state and schedules
│   ├── enable <job> [--scope security|system] [--keep N]
│   └── disable <job>|all
│                                 # Jobs: updates, self-update,
│                                 #       kernel-cleanup, update-cache
├── migrate [--apply]             # Rewrite cron/Zabbix files using old names
├── zabbix                        # Zabbix integration
│   └── init                      # Initialize Zabbix support
├── self-update [--check] [--force]
│                                 # Update from the latest GitHub release
└── version [--verbose, -v]       # Show version info
```

## Output format

`-o, --output` is a persistent flag available on every command, replacing the per-command
`--json` and `--json-pretty` booleans.

| Value | Meaning |
|-------|---------|
| `text` | Human-readable |
| `json` | Compact JSON |
| `json-pretty` | Indented JSON |

Each command keeps its historical default, so callers that pass nothing see what they always
saw: `updates check` and `kernel status` default to `json`; `banner` and `users list` default
to `text`.

Values complete with `<TAB>`, as do `schedule enable|disable` job names and `--scope` values.
The installers write the completion script to `/etc/bash_completion.d/syschecks` and
regenerate it on every upgrade.

## Deprecated spellings

All still work, and emit a deprecation notice on **stderr** so JSON on stdout stays clean.

| Old | New |
|-----|-----|
| `apply-updates` | `updates apply --scope security` |
| `apply-updates --system` / `-s` | `updates apply --scope system` |
| `apply-updates --ignore-lock-file` / `-i` | `updates apply --ignore-locks` |
| `updates` | `updates check` |
| `updates --cache-create` | `updates refresh` |
| `updates --cache-use=false` | `updates check --refresh` |
| `kernel` | `kernel status` |
| `kernel cleanup --execute` | `kernel cleanup` |
| `userinfo` | `users list` |
| `sysinfo` | `banner --output json` |
| `cron` | `schedule` |
| `cron status` | `schedule list` |
| `cron init [--disable]` | `schedule enable\|disable update-cache` |
| `cron updates --security\|--system` | `schedule enable updates --scope security\|system` |
| `cron updates --disable` | `schedule disable updates` |
| `cron autoupdate [--disable]` | `schedule enable\|disable self-update` |
| `cron kernels [--disable]` | `schedule enable\|disable kernel-cleanup` |
| `--json` / `--json-pretty` | `--output json` / `--output json-pretty` |
| `banner --no-emojies` | `banner --no-emojis` |

## Breaking change: `kernel cleanup`

`kernel cleanup` **removes** old kernel packages by default. It previously previewed unless
`--execute` was passed.

- `--dry-run` gives the old preview behaviour.
- `--execute` is accepted and ignored; existing cron files keep working.
- Run interactively without `--yes`, it lists the packages and asks for confirmation. Only
  `y`/`yes` proceeds; a blank line, EOF, or an unreadable stdin aborts.
- Unattended runs (no TTY, i.e. cron) proceed without prompting.

## Migration

Generated files on already-deployed hosts embed literal command strings, so upgrading the
binary is not enough:

- `/etc/cron.d/syschecks_*` and legacy `/etc/cron.d/automatic_*` job files
- `/etc/zabbix/zabbix_agentd.conf` / `zabbix_agent2.conf` `UserParameter` lines

```bash
syschecks migrate           # report only; exits non-zero while changes are pending
syschecks migrate --apply   # rewrite
```

Report-only is the default so nothing is rewritten unattended. `auto-install.sh` and
`install-offline.sh` call `syschecks migrate --apply` as their final step. Re-running is a
no-op, and a fresh install never needs it.

This is a command rather than a check inside the binary on purpose: `banner` runs on every
SSH login, and spending even 100-200 ms there to fix a one-time problem would tax every
login forever.

## Detailed Command Reference

### `syschecks kernel status`

Check if a kernel reboot is needed. Compares running kernel against installed kernels.

**Flags:**
- `--output json-pretty` - Format JSON output with indentation

**Output:** JSON object with:
```json
{
  "kernel_needs_reboot": true,
  "running_kernel": "5.15.0-91",
  "latest_installed_kernel": "5.15.0-92",
  "list_of_installed_kernels": ["5.15.0-91", "5.15.0-92"],
  "installed_kernel_count": 2
}
```

**Aliases:** `kern`

---

### `syschecks kernel cleanup`

**Removes** old kernel packages. The running kernel and a recent fallback are always protected.

**Flags:**
- `--keep N` - Number of kernels to keep (default: 4, including running kernel)
- `--dry-run` - Show what would be removed without removing anything
- `--yes`, `-y` - Skip the interactive confirmation
- `--execute` - Deprecated; accepted and ignored (removal is now the default)

**Requires:** Root privileges, unless `--dry-run`

**Aliases:** `clean`, `cl`

---

### `syschecks updates check`

Check for available system and security updates.

**Flags:**
- `--cached` - Read the cached report (default)
- `--refresh` - Query the package manager directly, refreshing repository health too

**Output:** JSON object with update information

**Notes:**
- First run may be slow as it refreshes package cache
- Cache stored at `/tmp/syscheck_updates.json`

---

### `syschecks updates apply`

Apply pending updates using the system package manager.

**Flags:**
- `--system, -s` - Apply all system updates (default: security only)
- `--ignore-lock-file, -i` - Ignore package lock file

**Requires:** Root privileges

**Notes:**
- Respects `/opt/syschecks/package.lock.json` for excluded packages
- 10-minute timeout per package installation
- Automatically refreshes update cache after completion

---

### `syschecks banner`

Display a formatted system information banner. Designed for SSH login screens.

**Flags:**
- `--all` - Show healthy and normally suppressed details, including all writable filesystems
- `--no-emojies, -n` - Disable emoji characters
- `--disk-used-threshold PERCENT` - Warn above this used-space percentage (default: 90)

**Displays:**
- OS name and version
- Hostname and IP addresses
- System uptime
- Number of logged-in users and sessions, only when more than one user is active
- CPU and RAM information
- Low-space disk warnings only when a writable filesystem is at least 90% full
- Kernel reboot warning; installed-kernel count only above six
- Pending update counts, only when non-zero
- Installed version and self-update state in the top-right border
- Automatic OS-update problems; fully disabled and conflicting modes are highlighted red while healthy modes stay hidden

**Usage in login:**
```bash
echo '([ -z "$PS1" ] && true) || syschecks banner' >> /etc/profile.d/syschecks_banner.sh
```

---

### `syschecks schedule` / `syschecks schedule list`

Display a read-only dashboard of every available SysChecks cron job. The table
shows whether each job is enabled, disabled, inactive, legacy, or conflicting;
its configured schedule in server local time; and the relevant management
command. Disabled jobs show the schedule they will receive when enabled.

If security-only and full-system updates are both active, both table rows are
marked `CONFLICT` and a remediation warning is printed.

---

### `syschecks schedule enable|disable update-cache`

Schedule periodic refreshes of the update cache that the banner and monitoring read.

```bash
syschecks schedule enable update-cache
syschecks schedule disable update-cache
```

**Creates:** `/etc/cron.d/syschecks_cache`

**Schedule:**
- On boot (with random delay)
- Every 12 hours at minute 7

**Requires:** Root privileges

---

### `syschecks schedule enable|disable updates`

Schedule automatic update installation. `--scope` is required: the command never guesses a
host's update policy.

```bash
syschecks schedule enable updates --scope security
syschecks schedule enable updates --scope system
syschecks schedule disable updates
```

**Flags:**
- `--scope security|system` - Required. Which updates to install automatically
- `--disable` - Remove both automatic update cron jobs

Security-only and full-system modes are mutually exclusive. Enabling either
mode removes the other job and prints a warning identifying the removed file.
Passing more than one mode flag is rejected. Legacy `automatic_*` update jobs
are removed with warnings when a mode is enabled or disabled.

**Creates:** 
- `/etc/cron.d/syschecks_updates_security` (for --security)
- `/etc/cron.d/syschecks_updates_system` (for --system)

**Schedule:** Daily at 4:15 AM (with random delay)

**Log:** `/var/log/syschecks_updates.log`

**Requires:** Root privileges

---

### `syschecks schedule enable|disable self-update`

Schedule a cron job that keeps syschecks updated to the latest GitHub release.

**Flags:**
- `--disable` - Remove the auto-update cron job

**Creates:** `/etc/cron.d/syschecks_autoupdate` (runs `syschecks self-update`)

**Schedule:** Daily at 3:30 AM (with random delay)

**Log:** `/var/log/syschecks_selfupdate.log`

**Requires:** Root privileges

---

### `syschecks schedule enable|disable kernel-cleanup`

Schedule weekly removal of old kernel packages.

**Flags:**
- `--keep N` - Retain at least this many kernels (default: 4; minimum: 2)
- `--disable` - Remove the kernel-cleanup cron job

**Creates:** `/etc/cron.d/syschecks_kernel_cleanup`

**Schedule:** Sundays at 3:45 AM (with random delay)

**Log:** `/var/log/syschecks_kernel_cleanup.log`

**Requires:** Root privileges

---

### `syschecks zabbix init`

Configure Zabbix agent to work with syschecks.

**Modifies:** `/etc/zabbix/zabbix_agentd.conf` or `/etc/zabbix_agentd.conf`

**Adds:** UserParameter for syschecks integration:
```
UserParameter=syschecks[*],syschecks $1
```

**Actions:**
1. Removes any existing syschecks integration lines
2. Adds new UserParameter
3. Restarts zabbix-agent service

**Requires:** Root privileges

---

### `syschecks banner --output json` (replaces `sysinfo`)

Output system IP addresses in JSON format.

**Output:**
```json
{"ip_address_list": "192.168.1.100, 10.0.0.1"}
```

---

### `syschecks users list`

List real system users in a formatted table.

**Flags:**
- `--json` - Output user information as compact JSON
- `--output json-pretty` - Output user information as formatted JSON
- `--all` - Include system and no-login users

**Default filtering:**
- Includes `root`
- Includes users in the normal UID range from `/etc/login.defs`
- Excludes no-login service accounts unless `--all` is set

**Displays:**
- Username and UID in the compact table; shell and account details in JSON
- Whether the user currently has active sessions
- Active session source such as SSH, getty/console, local graphical, terminal, or tmux
- Password status from `/etc/shadow` when run as root
- Last login time, host, TTY, and inferred source from `last -w -F`

**Notes:**
- Non-root users normally cannot read `/etc/shadow`; password status will be `unknown (requires root)`.
- Login source is inferred from `who` and `last` TTY/host data.

---

### `syschecks self-update`

Update the installed binary to the latest GitHub release.

**Flags:**
- `--check` - Only report whether an update is available; do not install
- `--force` - Reinstall the latest release even if the version already matches

**Behavior:**
1. Queries the latest release from the GitHub API.
2. No-op when the installed version already matches the latest release (unless `--force`); never downgrades.
3. Downloads the asset for the running OS/arch (`syschecks-<os>-<arch>`) into the target directory.
4. Verifies the published SHA-256 from `checksums-sha256.txt` when present.
5. Atomically replaces the running binary (resolving symlinks such as `/bin/syschecks`), preserving file mode and ownership.

**Environment:**
- `GITHUB_TOKEN` (optional) - raises the GitHub API rate limit / allows private repos.

**Requires:** Root privileges (to replace the binary under `/opt/syschecks`).

---

### `syschecks version`

Display version information.

**Flags:**
- `--verbose, -v` - Show detailed version including commit, build date, Go version, and platform

**Examples:**
```bash
$ syschecks version
syschecks v1.2.3

$ syschecks version -v
Version: v1.2.3
Commit: abc12345
Built: 2025-01-08_12:00:00_UTC
Go: go1.21.5
Platform: linux/amd64
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error / Not running as root when required |

## Environment

The tool reads system files directly:
- `/etc/os-release` - OS identification
- `/proc/meminfo` - Memory information
- `/boot/` - Installed kernels

Package management commands used:
- **apt/apt-get** - Debian/Ubuntu
- **yum** - CentOS
- **dnf** - RHEL 8+, AlmaLinux, Rocky Linux
