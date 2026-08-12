# SysChecks v2

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Linux](https://img.shields.io/badge/OS-Linux-FCC624?logo=linux&logoColor=black)](https://www.linux.org/)

SysChecks is a Linux CLI for small operational checks and maintenance tasks. It provides JSON-friendly output for monitoring systems and a few host maintenance helpers around updates, kernels, login banners, cron jobs, and Zabbix integration.

## Features

- Kernel reboot checks and old-kernel cleanup command generation
- System update checks for apk, apt, dnf, and yum systems, with security-only capability reported explicitly
- Optional update cache for fast monitoring reads
- Controlled update application with a package lock file
- Login banner with host, kernel, update, CPU, memory, and user-facing system data
- Real-user inventory with login state, password lock state, and last-login source
- Cron job generation for cache refresh and automatic updates
- Zabbix `UserParameter` integration

## Install

```bash
curl -sSL https://raw.githubusercontent.com/yaroslav-gwit/SysChecks_v2/main/auto-install.sh | sudo bash
```

Manual install:

```bash
VERSION="1.0.2"
wget "https://github.com/yaroslav-gwit/SysChecks_v2/releases/download/v${VERSION}/syschecks-linux-amd64"

sudo mkdir -p /opt/syschecks
sudo mv syschecks-linux-amd64 /opt/syschecks/syschecks
sudo chmod +x /opt/syschecks/syschecks
sudo ln -sf /opt/syschecks/syschecks /usr/bin/syschecks

sudo syschecks completion bash > /etc/bash_completion.d/syschecks
```

## Offline / Air-Gapped Install (via bastion)

For servers with no internet access, SysChecks ships a self-extracting `.run`
installer that bundles the binary, the package lock file, and the offline
installer into a single file. Copy it to the target host and run it as root —
nothing is downloaded on the target.

**Usecase:** the air-gapped servers have no internet, but an admin can reach
them over SSH from a bastion host that *does* have internet.

One-liner from the bastion (streams the installer straight to the target over SSH):

```bash
curl -fsSL https://github.com/yaroslav-gwit/SysChecks_v2/releases/latest/download/syschecks-installer-amd64.run \
  | ssh root@<remote_ip> 'cat > /tmp/syschecks-installer.run && bash /tmp/syschecks-installer.run && rm -f /tmp/syschecks-installer.run'
```

For `arm64` targets, swap in `syschecks-installer-arm64.run`.

Or, copy once and run (handy for multiple hosts or a fully offline bastion):

```bash
# On the bastion: fetch the installer once
curl -fsSLO https://github.com/yaroslav-gwit/SysChecks_v2/releases/latest/download/syschecks-installer-amd64.run

# Push it to each target and install
scp syschecks-installer-amd64.run root@<remote_ip>:/tmp/
ssh root@<remote_ip> 'bash /tmp/syschecks-installer.run'
```

Re-running the installer updates the binary in place and preserves your
existing `package.lock.json` (the new lock is staged as
`package.lock.latest.json` for review).

Inspect the bundle without installing:

```bash
./syschecks-installer-amd64.run --extract /tmp/syschecks-payload
./syschecks-installer-amd64.run --help
```

Build the `.run` yourself from a checkout (it is also produced automatically by
`./release.sh` and attached to each GitHub release):

```bash
./build-advanced.sh cross          # produce bin/syschecks-linux-{amd64,arm64}
./create-run-installer.sh --arch amd64
# -> bin/syschecks-installer-amd64.run
```

## Common Commands

```bash
# Kernel status
syschecks kernel --json-pretty

# Old kernel cleanup command generation
sudo syschecks kernel cleanup --keep 4
# Execute the reviewed cleanup plan
sudo syschecks kernel cleanup --execute --keep 4

# Update checks
sudo syschecks updates --cache-create
syschecks updates --cache-use=true --json-pretty
sudo syschecks updates --cache-use=false --json-pretty

# Apply updates
sudo syschecks apply-updates
sudo syschecks apply-updates --system
sudo syschecks apply-updates --ignore-lock-file

# Host/user info
syschecks banner
syschecks banner --no-emojies
syschecks banner --all              # Include healthy/suppressed details
syschecks banner --disk-used-threshold 90
syschecks sysinfo
sudo syschecks userinfo
sudo syschecks userinfo --json-pretty

# Cron and Zabbix setup
syschecks cron                     # Show job state and schedules
sudo syschecks cron init
sudo syschecks cron updates --security
sudo syschecks cron autoupdate
sudo syschecks cron kernels --keep 4
# Every cron type has a matching removal path
sudo syschecks cron init --disable
sudo syschecks cron updates --disable
sudo syschecks cron autoupdate --disable
sudo syschecks cron kernels --disable
sudo syschecks zabbix init

# Self-update from the latest GitHub release
sudo syschecks self-update --check
sudo syschecks self-update
```

Security-only and full-system update schedules are mutually exclusive. Enabling
one removes the other—and any legacy duplicate schedule—with a CLI warning.

On Alpine, install and enable `cronie` before enabling SysChecks schedules; the
default BusyBox `crond` does not read `/etc/cron.d`. Alpine's apk client has no
security-only advisory channel, so security counts/scopes are reported as
unsupported rather than zero. Full-system checks and updates remain available.

## Command Tree

```text
syschecks
├── kernel [--json-pretty]
│   └── cleanup [--keep N] [--execute]
├── updates [--json-pretty] [--cache-create] [--cache-use]
├── apply-updates [--system] [--ignore-lock-file]
├── banner [--all] [--no-emojies] [--disk-used-threshold PERCENT]
├── cron
│   ├── status
│   ├── init [--disable]
│   ├── updates [--security|--system|--disable]
│   ├── autoupdate [--disable]
│   └── kernels [--keep N] [--disable]
├── zabbix
│   └── init
├── sysinfo
├── userinfo [--json|--json-pretty] [--all]
├── self-update [--check] [--force]
└── version [--verbose]
```

## Configuration

Package lock file:

```text
/opt/syschecks/package.lock.json
```

Example:

```json
[
  "docker-ce",
  "docker-ce-cli",
  "nvidia-driver",
  "linux-headers"
]
```

Other paths:

| Path | Purpose |
| --- | --- |
| `/tmp/syscheck_updates.json` | Update check cache |
| `/etc/cron.d/syschecks_cache` | Cache refresh cron job |
| `/etc/cron.d/syschecks_updates_security` | Automatic security update cron job |
| `/etc/cron.d/syschecks_updates_system` | Automatic system update cron job |
| `/var/log/syschecks_updates.log` | Automatic update log |

## Supported Systems

| Distribution family | Package manager | Notes |
| --- | --- | --- |
| Alpine Linux | apk | Full-system updates; security-only status is explicit `unsupported`; scheduling requires `cronie` |
| Ubuntu, Debian, Pop!_OS, Linux Mint, Elementary, Kali, Raspberry Pi OS, Zorin, KDE neon | apt | Uses `apt-get` |
| RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Fedora, Amazon Linux, openEuler | dnf/yum | Prefers `dnf` when available |
| CentOS 7 | yum | Legacy path |

Package manager detection uses `/etc/os-release` `ID` and `ID_LIKE`, then verifies the available `apk`, `apt-get`, `dnf`, or `yum` binary.

## Zabbix

```bash
sudo syschecks zabbix init
```

The command adds:

```ini
UserParameter=syschecks[*],syschecks $1
```

Example item keys:

- `syschecks[kernel]`
- `syschecks[updates]`
- `syschecks[sysinfo]`

## Login Banner

```bash
echo '([ -z "$PS1" ] && true) || syschecks banner' | sudo tee /etc/profile.d/syschecks_banner.sh
sudo chmod +x /etc/profile.d/syschecks_banner.sh
```

## Build

Requirements:

- Go 1.21+
- Git
- Docker, for compatibility builds

```bash
go test ./...
./build.sh
./bin/syschecks version
```

Compatibility build:

```bash
./docker-build.sh
```

Advanced build options:

```bash
./build-advanced.sh docker ubuntu18
./build-advanced.sh static
./build-advanced.sh cross
```

## Documentation

- [Changelog](CHANGELOG.md)
- [Releases Guide](RELEASES.md)
- [Architecture Overview](AgentDocs/ARCHITECTURE.md)
- [Command Reference](AgentDocs/COMMANDS.md)
- [Development Guide](AgentDocs/DEVELOPMENT.md)
- [E2E Test Plan](AgentDocs/E2E_TESTING.md)
- [Zabbix Integration](AgentDocs/ZABBIX_INTEGRATION.md)
- [Project Constitution](AgentDocs/CONSTITUTION.md)

## Troubleshooting

Refresh a stale update cache:

```bash
sudo syschecks updates --cache-create
```

Check package lock file:

```bash
ls -la /opt/syschecks/package.lock.json
cat /opt/syschecks/package.lock.json
```

Verify Zabbix integration:

```bash
grep syschecks /etc/zabbix/zabbix_agentd.conf
zabbix_get -s 127.0.0.1 -k "syschecks[kernel]"
```

## License

MIT. See [LICENSE](LICENSE).
