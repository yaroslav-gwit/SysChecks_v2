# SysChecks v2 - Claude AI Agent Guide

## Project Overview

SysChecks v2 is a Go-based CLI tool for Linux system administration, primarily extending Zabbix functionality. It provides system monitoring, update management, kernel checks, and automated maintenance features.

## Quick Reference

| Aspect | Details |
|--------|---------|
| Language | Go 1.21+ |
| CLI Framework | [cobra](https://github.com/spf13/cobra) |
| Target OS | Linux (Debian/Ubuntu, RHEL/CentOS/AlmaLinux/Rocky) |
| Binary Name | `syschecks` |
| Install Location | `/bin/syschecks` and `/opt/syschecks/` |

## Build Commands

```bash
# Standard build
./build.sh

# Advanced build with cross-compilation
./build-advanced.sh

# Docker-based build (for portable binaries)
./docker-build.sh

# Create a release (with dry-run first)
./release.sh -n 1.0.0    # Dry run
./release.sh 1.0.0       # Actual release
./release.sh -f 1.0.0    # Skip confirmations
```

### Release Script Options

| Option | Description |
|--------|-------------|
| `-n, --dry-run` | Simulate release without changes |
| `-f, --force` | Skip confirmation prompts |
| `-s, --skip-build` | Use existing binaries |
| `-t, --skip-tests` | Skip running tests |
| `-v, --verbose` | Enable verbose output |

## Project Structure

```
├── main.go              # Entry point - calls cmd.Execute()
├── cmd/                 # All CLI commands (cobra-based)
│   ├── root.go          # Root command + CLI initialization
│   ├── kernel.go        # Kernel reboot checks & cleanup
│   ├── check_updates.go # System/security update detection
│   ├── apply_updates.go # Update installation
│   ├── loginBanner.go   # SSH login banner display
│   ├── cron.go          # Cron job management
│   ├── zabbix.go        # Zabbix agent integration
│   ├── sysinfo.go       # System info (IP addresses)
│   ├── userinfo.go      # User listing (stub)
│   └── version.go       # Version info with build metadata
├── helpers/             # Shared utility functions
│   ├── generalHelpers.go # OS detection, RAM/CPU info, root check
│   ├── templates.go      # Cron job templates
│   └── zabbixInit.go     # Zabbix config modification
├── bin/                 # Compiled binaries
├── docker/              # Docker build configurations and docs
└── shell/               # Legacy shell scripts (deprecated)
```

## Key Commands

Commands follow `syschecks <resource> <verb>`. Every pre-v1.3.0 spelling still works as a
hidden, deprecated alias — see `syschecks migrate`.

| Command | Description |
|---------|-------------|
| `syschecks banner` | Display SSH login banner (stays top level: `/etc/profile.d` invokes it) |
| `syschecks banner -o json` | Full machine-readable system report, every check including healthy ones |
| `syschecks updates check` | Report available updates (cached; `--refresh` for a live query) |
| `syschecks updates apply --scope security\|system` | Install updates (`--dry-run` to preview) |
| `syschecks updates refresh` | Refresh the update cache (also `updates cache refresh`) |
| `syschecks kernel status` | Check if a kernel reboot is needed |
| `syschecks kernel cleanup` | **Removes** old kernel packages (`--dry-run` to preview) |
| `syschecks users list` | List users and who is logged in |
| `syschecks schedule list` | Show which jobs run automatically |
| `syschecks schedule enable <job>` | Enable a job: `updates` (needs `--scope`), `self-update`, `kernel-cleanup`, `update-cache` |
| `syschecks schedule disable <job>` | Disable a job, or `all` |
| `syschecks migrate [--apply]` | Rewrite cron/Zabbix files that use old command names |
| `syschecks zabbix init` | Configure Zabbix agent integration |
| `syschecks version` | Show version (`-v` for verbose) |

### Global flags

`-o, --output text|json|json-pretty` works on every command and replaces the old per-command
`--json` / `--json-pretty`. Values, job names, and `--scope` values are all tab-completable;
the installers write the completion script to `/etc/bash_completion.d/syschecks`.

### Deprecated spellings (still functional)

| Old | New |
|-----|-----|
| `apply-updates [--system]` | `updates apply [--scope system]` |
| `updates --cache-create` | `updates refresh` |
| `kernel` | `kernel status` |
| `kernel cleanup --execute` | `kernel cleanup` (removal is the default now) |
| `userinfo` | `users list` |
| `sysinfo` | `banner -o json` |
| `cron status` | `schedule list` |
| `cron init\|updates\|autoupdate\|kernels [--disable]` | `schedule enable\|disable <job>` |

## Coding Patterns

### Command Structure (Cobra)
All commands follow this pattern in `cmd/`:
```go
var cmdName = &cobra.Command{
    Use:   "name",
    Short: "Brief description",
    Long:  "Detailed description",
    Run: func(cmd *cobra.Command, args []string) {
        // Implementation
    },
}
```

### OS Detection
The codebase supports multiple Linux distros via `/etc/os-release` parsing:
- **Debian-based**: Ubuntu, Pop!_OS, Debian → uses `apt`
- **RHEL-based**: CentOS → uses `yum`; AlmaLinux, Rocky, Oracle, RHEL → uses `dnf`

### JSON Output
Many commands output JSON for Zabbix integration:
```go
jsonMarshal, _ := json.Marshal(data)
fmt.Println(string(jsonMarshal))
// Or with --json-pretty flag:
jsonMarshalIndent, _ := json.MarshalIndent(data, "", "   ")
```

### Root User Checks
Commands requiring root privileges call:
```go
helpers.RootUserCheck()  // Exits if not root
```

## Dependencies

Key external packages (from `go.mod`):
- `github.com/spf13/cobra` - CLI framework
- `github.com/briandowns/spinner` - Progress spinners
- `github.com/Delta456/box-cli-maker/v2` - Box drawing for banner
- `github.com/facette/natsort` - Natural sorting for kernel versions
- `github.com/gookit/color` / `github.com/fatih/color` - Terminal colors

## Important Files

- `/opt/syschecks/package.lock.json` - Package lock list (skipped during updates)
- `/tmp/syscheck_updates.json` - Cached update information
- `/etc/cron.d/syschecks_*` - Cron jobs created by the tool
- `/etc/zabbix/zabbix_agentd.conf` - Modified for Zabbix integration

## Testing Considerations

- The tool is designed for **Linux only** - macOS/Windows are not supported
- Many functions require root privileges
- Most commands interact with system utilities: `uname`, `dnf`, `apt`, `lscpu`, etc.
- Update checks cache results to `/tmp/` for performance

## Version Injection

Build with version metadata:
```bash
go build -ldflags="-X 'syschecks/cmd.Version=$VERSION' \
  -X 'syschecks/cmd.GitCommit=$COMMIT' \
  -X 'syschecks/cmd.BuildDate=$DATE'"
```
