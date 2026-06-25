# SysChecks End-to-End Release Test Plan

This document is a release validation runbook for SysChecks. It is intended for humans and LLM agents that need to verify the complete CLI before a release.

Run destructive tests only in disposable containers or VMs. Commands that create cron files, edit Zabbix config, apply updates, or test kernel cleanup must not be run on a workstation or production host unless explicitly approved.

## Scope

Validate every user-facing command and integration:

- Build, version metadata, help, and shell completion
- OS display name detection used by `syschecks banner`
- `banner` and `banner --no-emojies`
- `sysinfo`
- `userinfo`
- `kernel` reboot checks
- `kernel cleanup`
- `updates` fresh checks, cached checks, and cache creation
- `apply-updates` security-only, full-system, package lock, and `--ignore-lock-file`
- `cron init`
- `cron updates --security`
- `cron updates --system`
- `zabbix init`
- Supported package managers: apt, dnf, yum

## Required Test Environments

Use one test environment at a time to avoid stressing the host.

### Container Matrix

Containers are enough for most CLI, update, cron, Zabbix-file, and banner checks.

| Image | Package manager | Purpose |
| --- | --- | --- |
| `ubuntu:22.04` | apt | apt updates, apt apply, banner, cache |
| `ubuntu:26.04` | apt | future Ubuntu apt-get availability and OS name |
| `debian:12-slim` | apt | Debian no-update or low-update path |
| `fedora:40` | dnf | dnf `repoquery` and `updateinfo` paths |
| `almalinux:8` | dnf | EL dnf security advisories and security apply |
| `rockylinux:8` or `rockylinux:9` | dnf | Rocky/RHEL-like detection |
| `oraclelinux:8` | dnf | Oracle Linux detection |
| `centos:7` | yum | legacy yum path; requires vault repo rewrite |

### VM Matrix

Containers do not provide realistic `/boot`, bootloader state, running-kernel replacement, systemd service restart behavior, or Zabbix service behavior. Use disposable VMs for:

- Kernel reboot-required behavior
- Kernel cleanup package discovery and command safety
- Zabbix service restart
- Cron execution over time
- SSH login banner integration via `/etc/profile.d`

Minimum VM coverage:

| VM | Purpose |
| --- | --- |
| Ubuntu LTS VM | apt, kernel cleanup, SSH banner |
| AlmaLinux/Rocky/RHEL VM | dnf, RPM kernel package discovery, Zabbix service |
| CentOS 7 VM if supported | yum legacy behavior |

## Build Under Test

From the repository root:

```bash
set -euo pipefail

go test ./...

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo \
  -ldflags="-w -s \
    -X 'syschecks/cmd.Version=${VERSION}' \
    -X 'syschecks/cmd.GitCommit=${COMMIT}' \
    -X 'syschecks/cmd.BuildDate=${DATE}'" \
  -o /tmp/syschecks-e2e .

file /tmp/syschecks-e2e
/tmp/syschecks-e2e version
/tmp/syschecks-e2e version --verbose
```

Pass criteria:

- `go test ./...` succeeds.
- Binary is Linux amd64 and ideally statically linked.
- `version` prints `syschecks <version>` or `syschecks development`.
- `version --verbose` includes `Version:`, `Go:`, and `Platform:`.

## Generic Container Harness

Use this pattern to run one image at a time:

```bash
IMAGE="ubuntu:22.04"

sudo docker run --rm \
  -v /tmp/syschecks-e2e:/usr/local/bin/syschecks:ro \
  "$IMAGE" \
  bash -lc 'set -euo pipefail; syschecks version; syschecks --help'
```

If the image does not include `bash`, use:

```bash
sh -lc 'set -eu; syschecks version; syschecks --help'
```

For CentOS 7 containers, rewrite EOL repos before update tests:

```bash
sed -i s/mirror.centos.org/vault.centos.org/g /etc/yum.repos.d/*.repo
sed -i s/^#.*baseurl=http/baseurl=http/g /etc/yum.repos.d/*.repo
sed -i s/^mirrorlist=http/#mirrorlist=http/g /etc/yum.repos.d/*.repo
```

## Basic CLI Tests

Run in every container image:

```bash
syschecks --help
syschecks kernel --help
syschecks kernel cleanup --help
syschecks updates --help
syschecks apply-updates --help
syschecks banner --help
syschecks cron --help
syschecks cron init --help
syschecks cron updates --help
syschecks zabbix --help
syschecks zabbix init --help
syschecks sysinfo --help
syschecks userinfo --help
syschecks version --help
syschecks completion bash | head -20
```

Pass criteria:

- Every help command exits 0.
- Help text contains the command name and flags.
- `completion bash` produces shell completion text.

## JSON Shape Tests

Install `jq` where needed.

APT:

```bash
apt-get update
apt-get install -y jq ca-certificates
```

DNF:

```bash
dnf install -y jq ca-certificates
```

YUM:

```bash
yum install -y jq ca-certificates
```

Then run:

```bash
syschecks version --verbose

syschecks sysinfo | jq -e '
  has("ip_address_list") and
  (.ip_address_list | type == "string")
'

syschecks userinfo --json | jq -e '
  (type == "array") and
  (all(.[]; has("username") and has("uid") and has("active") and has("password_status") and has("last_login") and has("last_source")))
'
```

Pass criteria:

- `sysinfo` is valid JSON and includes `ip_address_list`.
- `userinfo --json` is valid JSON array.
- Each returned user object has `username`, `uid`, `active`, `password_status`, `last_login`, and `last_source`.

## Banner Tests

Run in containers and at least one VM:

```bash
syschecks banner
syschecks banner --no-emojies
```

Pass criteria:

- Command exits 0.
- Output includes:
  - `Welcome back`
  - `System info`
  - `OS installed:`
  - a real OS name, not `Unknown Linux distribution`
  - `Hostname:`
  - `Machine IPs:`
  - `System uptime:`
  - `CPU Info:`
  - `RAM Info`
  - `Kernel reboot status`
  - `Update status`
- `--no-emojies` output contains no emoji glyphs before labels.
- If `/tmp/syscheck_updates.json` does not exist or is stale, banner reports that the update cache is out-of-date.

Ubuntu 26.04-specific check:

```bash
cat /etc/os-release
command -v apt-get
apt-get --version | head -3
syschecks banner --no-emojies | grep -E 'Ubuntu 26.04|Ubuntu'
```

Pass criteria:

- `apt-get` exists.
- Banner detects Ubuntu 26.04 or Ubuntu from release metadata.

## Updates: Fresh Checks

Fresh update checks require root because package manager cache refreshes require root.

APT images:

```bash
syschecks updates --cache-use=false --json-pretty | tee /tmp/updates.json
jq -e '
  (.system_updates | type == "number") and
  (.security_updates | type == "number") and
  (.system_updates_available | type == "boolean") and
  (.security_updates_available | type == "boolean") and
  (.system_updates_list | type == "array") and
  (.security_updates_list | type == "array") and
  (.cache_exists == false) and
  (.cache_up_to_date == false)
' /tmp/updates.json
```

DNF images:

```bash
syschecks updates --cache-use=false --json-pretty | tee /tmp/updates.json
jq -e '
  (.system_updates_list | type == "array") and
  (.security_updates_list | type == "array")
' /tmp/updates.json
```

YUM/CentOS 7 image:

```bash
# Apply vault repo rewrite first.
syschecks updates --cache-use=false --json-pretty | tee /tmp/updates.json
jq -e '
  (.system_updates_list | type == "array") and
  (.security_updates_list | type == "array")
' /tmp/updates.json
```

Pass criteria:

- JSON parses.
- Count fields are numeric.
- Available fields are booleans.
- List fields are arrays, never `null`.
- No package-manager progress or warning text corrupts JSON stdout.
- On Fedora/Alma/Rocky with advisory metadata, security update rows come from advisories and are deduped by package.

Known acceptable cases:

- Fully current images may report 0 updates.
- CentOS 7 vault may report 0 security updates because security advisory metadata can be absent.

## Updates: Cache Creation and Cache Use

Run in each package-manager family:

```bash
rm -f /tmp/syscheck_updates.json

syschecks updates --cache-create
test -f /tmp/syscheck_updates.json
stat -c '%a %n' /tmp/syscheck_updates.json

syschecks updates --cache-use=true --json-pretty | tee /tmp/cache.json
jq -e '
  (.cache_exists == true) and
  (.cache_created_on | type == "string") and
  (.system_updates_list | type == "array") and
  (.security_updates_list | type == "array")
' /tmp/cache.json
```

Pass criteria:

- Cache file exists at `/tmp/syscheck_updates.json`.
- File mode is readable by non-root, expected `644`.
- Cached output includes `cache_exists: true`.
- Cached output includes `cache_created_on`.
- Cached list fields are arrays.

Stale cache test:

```bash
touch -d '2 days ago' /tmp/syscheck_updates.json
syschecks updates --cache-use=true --json-pretty | jq -e '.cache_up_to_date == false'
```

Pass criteria:

- Cache older than 12 hours is reported stale.

## Apply Updates: Package Lock

Run in a disposable container with available updates.

Create a package lock for a package known to be upgradable. Examples:

Ubuntu 22.04:

```bash
mkdir -p /opt/syschecks
printf '["libssl3"]\n' > /opt/syschecks/package.lock.json
syschecks apply-updates --system 2>&1 | tee /tmp/apply-lock.log
grep -q 'locked in package.lock.json' /tmp/apply-lock.log
```

AlmaLinux 8:

```bash
mkdir -p /opt/syschecks
printf '["openssl-libs"]\n' > /opt/syschecks/package.lock.json
syschecks apply-updates 2>&1 | tee /tmp/apply-lock.log
grep -q 'locked in package.lock.json' /tmp/apply-lock.log
```

Pass criteria:

- Locked package is skipped.
- Other packages may be upgraded.
- Command exits 0 even when a package is skipped.

Override test:

```bash
syschecks apply-updates --system --ignore-lock-file 2>&1 | tee /tmp/apply-ignore-lock.log
! grep -q 'locked in package.lock.json' /tmp/apply-ignore-lock.log
```

Pass criteria:

- Lock file is ignored when `--ignore-lock-file` is set.

## Apply Updates: Security Only

Run only in disposable containers or VMs.

Recommended image: `almalinux:8`.

```bash
syschecks updates --cache-use=false --json-pretty | tee /tmp/before.json
jq '.security_updates, .security_updates_list' /tmp/before.json

syschecks apply-updates 2>&1 | tee /tmp/security-apply.log

grep -E 'No updates to apply|Package upgraded|Package skipped|Package error' /tmp/security-apply.log
test -f /tmp/syscheck_updates.json
```

Pass criteria:

- Default mode applies security updates only.
- Logs mention each attempted package as upgraded, skipped, errored, or no updates.
- Cache is refreshed after apply.
- No package outside the security update list is intentionally selected.

## Apply Updates: Full System

Run only in disposable containers or VMs.

Recommended image: `ubuntu:22.04`.

```bash
syschecks updates --cache-use=false --json-pretty | tee /tmp/before-system.json
jq '.system_updates, .system_updates_list' /tmp/before-system.json

syschecks apply-updates --system 2>&1 | tee /tmp/system-apply.log

grep -E 'No updates to apply|Package upgraded|Package skipped|Package error' /tmp/system-apply.log
test -f /tmp/syscheck_updates.json
```

Pass criteria:

- `--system` applies packages from the system update list.
- Cache is refreshed after apply.
- Command exits 0 unless a fatal package-manager/setup error occurs.

## Kernel Status

Run in every container and VM:

```bash
syschecks kernel --json-pretty | tee /tmp/kernel.json
jq -e '
  (.reboot_required | type == "boolean") and
  (.running_kernel | type == "string") and
  (.latest_installed_kernel | type == "string")
' /tmp/kernel.json
```

Pass criteria:

- JSON parses.
- `reboot_required` is boolean.
- `running_kernel` is non-empty.
- `latest_installed_kernel` is non-empty.
- `list_of_installed_kernels`, when present, is an array.

Container note:

- Containers share the host kernel and may have no meaningful `/boot` kernel list. The command should still exit 0 and fall back safely.

## Kernel Cleanup

Run command-generation tests in containers, but run real cleanup only in disposable VMs.

Container smoke test:

```bash
syschecks kernel cleanup --keep 4 | tee /tmp/kernel-cleanup.txt
grep -E '# No old kernels found|# Found|# Run the following commands' /tmp/kernel-cleanup.txt
```

Pass criteria:

- Command exits 0.
- Output is comments plus suggested package-manager commands.
- It does not remove anything by itself.

VM setup for meaningful cleanup:

1. Install at least 3 kernel package versions.
2. Reboot into one of the kernels.
3. Confirm `/boot/vmlinuz-*` has multiple entries.
4. Run:

```bash
syschecks kernel --json-pretty
syschecks kernel cleanup --keep 2 | tee /tmp/kernel-cleanup.txt
```

APT pass criteria:

- Output suggests `sudo apt purge -y ...` only for installed packages matching old kernel versions.
- Package names may include:
  - `linux-image-*`
  - `linux-headers-*`
  - `linux-modules-*`
  - `linux-modules-extra-*`
  - `pve-kernel-*`
  - `proxmox-kernel-*`
- Running kernel is never selected for cleanup.
- Newest kept kernels are not selected.

DNF/YUM pass criteria:

- Output suggests `sudo dnf remove -y ...` or `sudo yum remove -y ...`.
- RPM package names are discovered from `rpm -qf /boot/...` and `rpm -qa`, not synthesized from hard-coded names.
- Running kernel is never selected for cleanup.
- Newest kept kernels are not selected.

Optional destructive VM cleanup:

```bash
# Read /tmp/kernel-cleanup.txt first.
# Execute only the generated package-manager command in a disposable VM.
sudo apt purge -y <packages>
# or
sudo dnf remove -y <packages>
# or
sudo yum remove -y <packages>

syschecks kernel cleanup --keep 2
```

Pass criteria:

- Generated packages are removed.
- System remains bootable after reboot.
- Running kernel remains installed.

## Cron Tests

Run in disposable containers or VMs as root.

```bash
rm -f /etc/cron.d/syschecks_cache \
      /etc/cron.d/syschecks_updates_security \
      /etc/cron.d/syschecks_updates_system

syschecks cron init
test -f /etc/cron.d/syschecks_cache
stat -c '%a %n' /etc/cron.d/syschecks_cache
grep -q 'syschecks updates --cache-create' /etc/cron.d/syschecks_cache

syschecks cron updates --security
test -f /etc/cron.d/syschecks_updates_security
test ! -f /etc/cron.d/syschecks_updates_system
grep -q 'syschecks apply-updates' /etc/cron.d/syschecks_updates_security

syschecks cron updates --system
test -f /etc/cron.d/syschecks_updates_system
test ! -f /etc/cron.d/syschecks_updates_security
grep -q 'syschecks apply-updates --system' /etc/cron.d/syschecks_updates_system
```

Pass criteria:

- Cron files are created with mode `644`.
- `cron init` creates cache cron and does not require update cron selection.
- `cron updates --security` removes conflicting system update cron.
- `cron updates --system` removes conflicting security update cron.
- Cron commands reference `syschecks` and expected flags.

Help/no-flag behavior:

```bash
syschecks cron updates
```

Pass criteria:

- Prints help and exits 0.
- Does not create update cron files.

## Zabbix Init Tests

Use a disposable VM for full service restart validation. Containers can validate config-file behavior with a fake `systemctl`.

### Container Config Test

```bash
mkdir -p /etc/zabbix /tmp/fakebin
cat > /etc/zabbix/zabbix_agentd.conf <<'EOF'
Server=127.0.0.1

# Old integration should be removed:
UserParameter=syschecks[*],old-command

Hostname=test-host
EOF

cat > /tmp/fakebin/systemctl <<'EOF'
#!/bin/sh
echo "$@" > /tmp/systemctl-called
exit 0
EOF
chmod +x /tmp/fakebin/systemctl
export PATH="/tmp/fakebin:$PATH"

syschecks zabbix init

grep -q '#_ SYSCHECKS INTEGRATION _#' /etc/zabbix/zabbix_agentd.conf
grep -q 'UserParameter=syschecks\[\*\],syschecks $1' /etc/zabbix/zabbix_agentd.conf
test "$(grep -ci 'syschecks' /etc/zabbix/zabbix_agentd.conf)" -eq 2
grep -q 'restart zabbix-agent' /tmp/systemctl-called
```

Pass criteria:

- Existing lines containing `syschecks` are removed.
- Exactly one new integration block is added.
- Fake `systemctl restart zabbix-agent` is called.

### VM Service Test

```bash
sudo apt-get install -y zabbix-agent || sudo dnf install -y zabbix-agent || sudo yum install -y zabbix-agent
sudo syschecks zabbix init
systemctl is-active zabbix-agent
grep -q 'UserParameter=syschecks\[\*\],syschecks $1' \
  /etc/zabbix/zabbix_agentd.conf /etc/zabbix_agentd.conf 2>/dev/null
```

Pass criteria:

- Zabbix config is updated.
- Zabbix agent restarts successfully.
- `systemctl is-active zabbix-agent` returns `active`.

## SSH Login Banner Integration Test

Run only in a disposable VM.

```bash
echo '([ -z "$PS1" ] && true) || syschecks banner' | sudo tee /etc/profile.d/syschecks_banner.sh
sudo chmod +x /etc/profile.d/syschecks_banner.sh

ssh localhost true
ssh -tt localhost 'exit' | tee /tmp/ssh-banner.txt
grep -q 'System info' /tmp/ssh-banner.txt
grep -q 'Kernel reboot status' /tmp/ssh-banner.txt
```

Pass criteria:

- Non-interactive SSH command is not polluted by banner output.
- Interactive SSH session displays banner.

Cleanup:

```bash
sudo rm -f /etc/profile.d/syschecks_banner.sh
```

## OS Detection Matrix

Run in each container image:

```bash
cat /etc/os-release || true
command -v apt-get || true
command -v dnf || true
command -v yum || true
syschecks banner --no-emojies | grep 'OS installed:'
syschecks updates --cache-use=false --json-pretty >/tmp/updates.json
```

Pass criteria:

- Debian-like systems select apt when `apt-get` exists.
- RHEL/Fedora-like systems select dnf when `dnf` exists.
- CentOS 7 selects yum when only yum is available.
- Banner OS name is readable and not the unknown fallback except on genuinely unknown test fixtures.

## Release Acceptance Checklist

Before cutting a release, record:

- Git commit tested:
- Binary path and checksum:
- `go test ./...` result:
- `go build` result:
- Container results:
  - Ubuntu 22.04:
  - Ubuntu 26.04:
  - Debian 12:
  - Fedora 40:
  - AlmaLinux 8:
  - Rocky Linux:
  - Oracle Linux:
  - CentOS 7:
- VM results:
  - Ubuntu:
  - Alma/Rocky/RHEL:
  - CentOS 7 if applicable:
- Update apply tests:
  - Security-only:
  - Full-system:
  - Package lock:
  - Ignore package lock:
- Kernel cleanup:
  - APT:
  - DNF/YUM:
- Cron:
- Zabbix:
- SSH banner:

Release is acceptable only when all required checks pass or skipped checks are explicitly documented with a reason.

## Cleanup After E2E Runs

Containers are removed with `--rm`. On VMs, clean up files created during tests:

```bash
sudo rm -f /tmp/syscheck_updates.json
sudo rm -f /etc/cron.d/syschecks_cache
sudo rm -f /etc/cron.d/syschecks_updates_security
sudo rm -f /etc/cron.d/syschecks_updates_system
sudo rm -f /var/log/syschecks_updates.log
sudo rm -f /etc/profile.d/syschecks_banner.sh
sudo rm -rf /opt/syschecks/package.lock.json
```

If Zabbix config was modified in a VM, restore the VM snapshot or restore the saved config backup.
