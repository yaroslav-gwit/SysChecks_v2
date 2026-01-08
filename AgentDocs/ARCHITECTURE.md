# SysChecks v2 Architecture

## High-Level Overview

```
┌─────────────────────────────────────────────────────────────┐
│                       main.go                                │
│                    (Entry Point)                             │
│                   cmd.Execute()                              │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      cmd/root.go                             │
│              (Root Command + CLI Init)                       │
│         Registers all subcommands via init()                 │
└─────────────────────────────┬───────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   kernel    │      │   updates   │      │   banner    │
│  commands   │      │  commands   │      │   command   │
└─────────────┘      └─────────────┘      └─────────────┘
         │                    │                    │
         └────────────────────┼────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     helpers/ package                         │
│           (Shared utilities and templates)                   │
└─────────────────────────────────────────────────────────────┘
```

## Package Structure

### `main` package (main.go)
- Single entry point
- Delegates immediately to `cmd.Execute()`
- Contains unused `IamRootCheck()` function (candidate for removal)

### `cmd` package
Primary command implementations using Cobra CLI framework.

#### root.go
- Defines the root `syschecks` command
- `init()` function registers ALL subcommands and their flags
- Central location for understanding the full CLI structure

#### kernel.go
- **Purpose**: Kernel version management
- **Key structs**:
  - `installedKernelsStruct` - Lists generic and OEM kernels
  - `compareKernelsStruct` - Comparison result with reboot status
- **Key functions**:
  - `getRunningKernel()` - Uses `uname -r`
  - `getInstalledKernels()` - Parses `/boot/` directory
  - `compareKernels()` - Determines if reboot is needed
  - `kernelCleanupAction()` - Generates cleanup commands

#### check_updates.go
- **Purpose**: Detect available system and security updates
- **Key struct**: `systemUpdatesStruct` - Update counts and lists
- **Key functions**:
  - `detectOs()` - Identifies package manager (apt/yum/dnf)
  - `dnfCheck()` / `yumCheck()` / `debCheck()` - Distro-specific update checking
  - `systemUpdates()` - Unified update checking with caching
- **Caching**: Results stored in `/tmp/syscheck_updates.json`

#### apply_updates.go
- **Purpose**: Install updates using system package manager
- **Key features**:
  - Respects package lock file (`/opt/syschecks/package.lock.json`)
  - 10-minute timeout per package
  - Supports `--ignore-lock-file` flag

#### loginBanner.go
- **Purpose**: Pretty SSH login banner with system info
- **Display includes**:
  - OS name, hostname, IP addresses
  - System uptime, CPU info, RAM usage
  - Kernel reboot status
  - Available updates

#### cron.go
- **Purpose**: Manage cron jobs for automated tasks
- **Subcommands**:
  - `cron init` - Create update cache cron job
  - `cron updates` - Enable automatic security/system updates

#### zabbix.go
- **Purpose**: Integrate with Zabbix monitoring agent
- **Action**: Modifies Zabbix agent config to add UserParameter

#### sysinfo.go / userinfo.go
- Lightweight commands for system/user information
- `userinfo` appears to be a stub (returns empty array)

#### version.go
- Version string management with build-time injection
- Supports verbose output with commit hash and build date

### `helpers` package

#### generalHelpers.go
- `PrettyOsName()` - Parses `/etc/os-release` for display name
- `RootUserCheck()` - Ensures running as root (exits if not)
- `GetRamInfoLinux()` - Parses `/proc/meminfo`
- `GetCpuInfoLinux()` - Parses `lscpu` output

#### templates.go
- Cron job template strings
- File path constants for cron jobs
- `CacheCreate()`, `SecurityUpdates()`, `SystemUpdates()` - Write cron files

#### zabbixInit.go
- `ZabbixInit()` - Modifies Zabbix agent configuration file
- Adds UserParameter for syschecks integration
- Restarts zabbix-agent service

## Data Flow Examples

### Update Check Flow
```
User runs: syschecks updates
         │
         ▼
cmd/check_updates.go: checkUpdates()
         │
         ├── If --cache-use: Read from /tmp/syscheck_updates.json
         │
         └── Otherwise:
               │
               ▼
         detectOs() → Determines apt/yum/dnf
               │
               ▼
         debCheck() / yumCheck() / dnfCheck()
               │
               ▼
         Return systemUpdatesStruct as JSON
```

### Banner Display Flow
```
User runs: syschecks banner
         │
         ▼
cmd/loginBanner.go: showLoginBanner()
         │
         ├── getUserName(), getHostName(), getIps()
         ├── helpers.PrettyOsName()
         ├── helpers.GetCpuInfoLinux()
         ├── helpers.GetRamInfoLinux()
         ├── compareKernels() (from kernel.go)
         └── systemUpdates() (from check_updates.go)
               │
               ▼
         box-cli-maker renders formatted output
```

## OS Compatibility Matrix

| Distro | Package Manager | Tested |
|--------|----------------|--------|
| Ubuntu | apt (deb) | ✅ |
| Debian | apt (deb) | ✅ |
| Pop!_OS | apt (deb) | ✅ |
| CentOS | yum | ✅ |
| AlmaLinux | dnf | ✅ |
| Rocky Linux | dnf | ✅ |
| Oracle Linux | dnf | ✅ |
| RHEL | dnf | ✅ |

## File System Interactions

| Path | Purpose | Created By |
|------|---------|------------|
| `/opt/syschecks/` | Installation directory | auto-install.sh |
| `/opt/syschecks/package.lock.json` | Package exclusion list | auto-install.sh |
| `/bin/syschecks` | Binary location | auto-install.sh |
| `/tmp/syscheck_updates.json` | Update cache | `updates --cache-create` |
| `/etc/cron.d/syschecks_*` | Cron job files | `cron` commands |
| `/etc/zabbix/zabbix_agentd.conf` | Zabbix config | `zabbix init` |
| `/etc/bash_completion.d/syschecks` | Bash completion | auto-install.sh |
| `/var/log/syschecks_updates.log` | Update log | Cron jobs |
