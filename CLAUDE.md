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

| Command | Description |
|---------|-------------|
| `syschecks kernel` | Check if kernel reboot is needed (JSON output) |
| `syschecks kernel cleanup` | Clean up old kernel packages |
| `syschecks updates` | Check for system/security updates |
| `syschecks apply-updates` | Apply security updates (or `--system` for all) |
| `syschecks banner` | Display SSH login banner |
| `syschecks cron init` | Set up update cache cron job |
| `syschecks cron updates --security/--system` | Enable automatic updates |
| `syschecks zabbix init` | Configure Zabbix agent integration |
| `syschecks version` | Show version (`-v` for verbose) |

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
