# SysChecks Command Reference

## Command Tree

```
syschecks
├── kernel [--json-pretty]        # Check kernel reboot status
│   └── cleanup [--keep N]        # Clean up old kernels
├── updates [--json-pretty] [--cache-create] [--cache-use]
│                                 # Check for updates
├── apply-updates [--system] [--ignore-lock-file]
│                                 # Apply updates
├── banner [--no-emojies, -n]     # Display login banner
├── cron                          # Cron job management
│   ├── init                      # Create cache update cron
│   └── updates [--security] [--system]
│                                 # Enable automatic updates
├── zabbix                        # Zabbix integration
│   └── init                      # Initialize Zabbix support
├── sysinfo                       # Show IP addresses (JSON)
├── userinfo                      # List users (JSON, stub)
└── version [--verbose, -v]       # Show version info
```

## Detailed Command Reference

### `syschecks kernel`

Check if a kernel reboot is needed. Compares running kernel against installed kernels.

**Flags:**
- `--json-pretty` - Format JSON output with indentation

**Output:** JSON object with:
```json
{
  "kernel_needs_reboot": true,
  "running_kernel": "5.15.0-91",
  "latest_installed_kernel": "5.15.0-92",
  "active_kernels": ["5.15.0-91", "5.15.0-92"]
}
```

**Aliases:** `kern`

---

### `syschecks kernel cleanup`

Generate commands to remove old kernel packages.

**Flags:**
- `--keep N` - Number of kernels to keep (default: 4, including running kernel)

**Aliases:** `clean`, `cl`

---

### `syschecks updates`

Check for available system and security updates.

**Flags:**
- `--json-pretty` - Pretty-print JSON output
- `--cache-create` - Create cache file for later use (requires root)
- `--cache-use` - Use cached results (default: true)

**Output:** JSON object with update information

**Notes:**
- First run may be slow as it refreshes package cache
- Cache stored at `/tmp/syscheck_updates.json`

---

### `syschecks apply-updates`

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
- `--no-emojies, -n` - Disable emoji characters

**Displays:**
- OS name and version
- Hostname and IP addresses
- System uptime
- CPU and RAM information
- Kernel reboot status
- Pending update counts

**Usage in login:**
```bash
echo '([ -z "$PS1" ] && true) || syschecks banner' >> /etc/profile.d/syschecks_banner.sh
```

---

### `syschecks cron init`

Create a cron job to update the syschecks cache periodically.

**Creates:** `/etc/cron.d/syschecks_cache`

**Schedule:**
- On boot (with random delay)
- Every 12 hours at minute 7

**Requires:** Root privileges

---

### `syschecks cron updates`

Enable automatic updates via cron jobs.

**Flags:**
- `--security` - Enable automatic security updates
- `--system` - Enable automatic system updates

**Creates:** 
- `/etc/cron.d/syschecks_updates_security` (for --security)
- `/etc/cron.d/syschecks_updates_system` (for --system)

**Schedule:** Daily at 4:15 AM (with random delay)

**Log:** `/var/log/syschecks_updates.log`

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

### `syschecks sysinfo`

Output system IP addresses in JSON format.

**Output:**
```json
{"ip_address_list": "192.168.1.100, 10.0.0.1"}
```

---

### `syschecks userinfo`

List system users in JSON format.

**Output:** Currently returns empty array `[]` (stub implementation)

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
