#!/bin/sh

# Non-destructive v1.2 banner/maintenance smoke test. This is designed to run
# as root inside one of the disposable containers from E2E_TESTING.md.
set -eu

SYSCHECKS_BIN="${SYSCHECKS_BIN:-syschecks}"

mkdir -p /etc/cron.d /boot
cron_files="/etc/cron.d/syschecks /etc/cron.d/automatic_security_updates /etc/cron.d/automatic_system_updates /etc/cron.d/automatic_system_updates_hold /etc/cron.d/syschecks_cache /etc/cron.d/syschecks_updates_security /etc/cron.d/syschecks_updates_system /etc/cron.d/syschecks_autoupdate /etc/cron.d/syschecks_kernel_cleanup"
rm -f $cron_files
trap 'rm -f $cron_files' EXIT

"$SYSCHECKS_BIN" version
"$SYSCHECKS_BIN" --help >/tmp/syschecks-help.txt
"$SYSCHECKS_BIN" cron >/tmp/syschecks-cron-status-disabled.txt
grep -q 'Update cache' /tmp/syschecks-cron-status-disabled.txt
grep -q 'Daily 04:15 (default)' /tmp/syschecks-cron-status-disabled.txt
"$SYSCHECKS_BIN" cron status >/tmp/syschecks-cron-status-explicit.txt
grep -q 'Kernel cleanup' /tmp/syschecks-cron-status-explicit.txt

"$SYSCHECKS_BIN" kernel --json-pretty >/tmp/syschecks-kernel.json
grep -q '"installed_kernel_count"' /tmp/syschecks-kernel.json

"$SYSCHECKS_BIN" kernel cleanup --keep 4 >/tmp/syschecks-kernel-cleanup.txt
grep -Eq '# No old kernels found|# Found' /tmp/syschecks-kernel-cleanup.txt

# Exercise --execute against fake package-manager binaries. No real package is
# changed; the fixture verifies that only the oldest fake kernel is selected.
running_kernel="$(cat /proc/sys/kernel/osrelease)"
touch "/boot/vmlinuz-$running_kernel" /boot/vmlinuz-0.0.1-syschecks-test /boot/vmlinuz-0.0.2-syschecks-test
test_bin=/tmp/syschecks-testbin
mkdir -p "$test_bin"

printf '%s\n' '#!/bin/sh' \
	'printf "linux-image-0.0.1-syschecks-test\nlinux-image-0.0.2-syschecks-test\n"' >"$test_bin/dpkg-query"
printf '%s\n' '#!/bin/sh' \
	'printf "%s\n" "$*" > /tmp/syschecks-package-manager-called' >"$test_bin/apt-get"
printf '%s\n' '#!/bin/sh' \
	'if [ "$1" = "-qa" ]; then printf "kernel-core-0.0.1-syschecks-test\nkernel-core-0.0.2-syschecks-test\n"; exit 0; fi' \
	'case "$*" in *0.0.1-syschecks-test*) printf "kernel-core-0.0.1-syschecks-test\n";; *) exit 1;; esac' >"$test_bin/rpm"
printf '%s\n' '#!/bin/sh' \
	'printf "%s\n" "$*" > /tmp/syschecks-package-manager-called' >"$test_bin/dnf"
cp "$test_bin/dnf" "$test_bin/yum"
chmod 0755 "$test_bin/dpkg-query" "$test_bin/apt-get" "$test_bin/rpm" "$test_bin/dnf" "$test_bin/yum"

PATH="$test_bin:$PATH" "$SYSCHECKS_BIN" kernel cleanup --execute --keep 2 >/tmp/syschecks-kernel-execute.txt
grep -q '0.0.1-syschecks-test' /tmp/syschecks-package-manager-called
if grep -q '0.0.2-syschecks-test' /tmp/syschecks-package-manager-called; then
	echo "Kernel cleanup selected a retained fallback kernel" >&2
	exit 1
fi
rm -rf "$test_bin" /tmp/syschecks-package-manager-called
rm -f "/boot/vmlinuz-$running_kernel" /boot/vmlinuz-0.0.1-syschecks-test /boot/vmlinuz-0.0.2-syschecks-test

"$SYSCHECKS_BIN" userinfo --json >/tmp/syschecks-users.json
grep -q '"last_login"' /tmp/syschecks-users.json

touch /etc/cron.d/syschecks
cache_output="$("$SYSCHECKS_BIN" cron init)"
printf '%s\n' "$cache_output" | grep -q 'Warning: removed duplicate or incompatible legacy cache cron job'
test -f /etc/cron.d/syschecks_cache
test ! -e /etc/cron.d/syschecks
"$SYSCHECKS_BIN" cron init --disable
test ! -e /etc/cron.d/syschecks_cache

touch /etc/cron.d/automatic_security_updates /etc/cron.d/automatic_system_updates /etc/cron.d/automatic_system_updates_hold
update_output="$("$SYSCHECKS_BIN" cron updates --security)"
printf '%s\n' "$update_output" | grep -q 'Warning: removed duplicate or incompatible legacy'
test -f /etc/cron.d/syschecks_updates_security
test ! -e /etc/cron.d/automatic_security_updates
test ! -e /etc/cron.d/automatic_system_updates
test ! -e /etc/cron.d/automatic_system_updates_hold
update_output="$("$SYSCHECKS_BIN" cron updates --system)"
printf '%s\n' "$update_output" | grep -q 'Warning: removed conflicting security-only updates cron job'
test -f /etc/cron.d/syschecks_updates_system
test ! -e /etc/cron.d/syschecks_updates_security

update_output="$("$SYSCHECKS_BIN" cron updates --security)"
printf '%s\n' "$update_output" | grep -q 'Warning: removed conflicting full system updates cron job'
test -f /etc/cron.d/syschecks_updates_security
test ! -e /etc/cron.d/syschecks_updates_system

if "$SYSCHECKS_BIN" cron updates --security --system >/tmp/syschecks-conflicting-cron.txt 2>&1; then
	echo "Contradictory update modes were accepted" >&2
	exit 1
fi
grep -q 'choose exactly one' /tmp/syschecks-conflicting-cron.txt

"$SYSCHECKS_BIN" cron updates --disable
test ! -e /etc/cron.d/syschecks_updates_security
test ! -e /etc/cron.d/syschecks_updates_system

printf '%s\n' '15 4 * * * root syschecks apply-updates' >/etc/cron.d/syschecks_updates_security
printf '%s\n' '15 4 * * * root syschecks apply-updates --system' >/etc/cron.d/syschecks_updates_system
"$SYSCHECKS_BIN" cron >/tmp/syschecks-cron-status-conflict.txt
test "$(grep -c 'CONFLICT' /tmp/syschecks-cron-status-conflict.txt)" = "2"
grep -q 'Warning: security-only and full-system update jobs are both active' /tmp/syschecks-cron-status-conflict.txt
"$SYSCHECKS_BIN" cron updates --disable >/dev/null

"$SYSCHECKS_BIN" cron autoupdate
"$SYSCHECKS_BIN" cron kernels --keep 4
test "$(stat -c %a /etc/cron.d/syschecks_kernel_cleanup)" = "644"
grep -q 'kernel cleanup --execute --keep 4' /etc/cron.d/syschecks_kernel_cleanup
"$SYSCHECKS_BIN" cron >/tmp/syschecks-cron-status-enabled.txt
grep -q 'Syschecks self-update.*enabled.*Daily 03:30' /tmp/syschecks-cron-status-enabled.txt
grep -q 'Kernel cleanup.*enabled.*Sunday 03:45' /tmp/syschecks-cron-status-enabled.txt

"$SYSCHECKS_BIN" banner --no-emojies --disk-used-threshold 100 >/tmp/syschecks-banner.txt
grep -q 'self-update:' /tmp/syschecks-banner.txt
grep -q 'Automatic OS updates:' /tmp/syschecks-banner.txt
grep -q 'OFF.*no scheduled system or security updates' /tmp/syschecks-banner.txt
if grep -q 'Logged-in users:' /tmp/syschecks-banner.txt; then
	echo "Single/zero logged-in user should be hidden" >&2
	exit 1
fi
if grep -q 'Installed kernels:' /tmp/syschecks-banner.txt; then
	echo "Healthy installed-kernel count should be hidden" >&2
	exit 1
fi
if grep -q 'No new .* updates available' /tmp/syschecks-banner.txt; then
	echo "Healthy update confirmations should be hidden" >&2
	exit 1
fi
if grep -q 'Low disk space' /tmp/syschecks-banner.txt; then
	echo "Unexpected disk warning at a 100% used threshold" >&2
	exit 1
fi

mkdir -p /tmp/syschecks-banner-testbin
printf '%s\n' '#!/bin/sh' \
	'printf "alice pts/0 2026-07-13 10:42 (192.0.2.10)\nbob pts/1 2026-07-13 10:43 (192.0.2.11)\n"' >/tmp/syschecks-banner-testbin/who
chmod 0755 /tmp/syschecks-banner-testbin/who
PATH="/tmp/syschecks-banner-testbin:$PATH" "$SYSCHECKS_BIN" banner --no-emojies --disk-used-threshold 100 >/tmp/syschecks-banner-two-users.txt
grep -q 'Logged-in users:.*2 (2 sessions)' /tmp/syschecks-banner-two-users.txt
rm -rf /tmp/syschecks-banner-testbin

touch "/boot/vmlinuz-$running_kernel"
for kernel_number in 1 2 3 4 5 6; do
	touch "/boot/vmlinuz-0.0.$kernel_number-syschecks-warning"
done
"$SYSCHECKS_BIN" banner --no-emojies --disk-used-threshold 100 >/tmp/syschecks-banner-kernel-warning.txt
grep -q 'Installed kernels: 7' /tmp/syschecks-banner-kernel-warning.txt
rm -f "/boot/vmlinuz-$running_kernel" /boot/vmlinuz-0.0.*-syschecks-warning

"$SYSCHECKS_BIN" banner --no-emojies --disk-used-threshold 0 >/tmp/syschecks-banner-low-disk.txt
grep -q 'Low disk space' /tmp/syschecks-banner-low-disk.txt

printf '%s\n' '{"system_updates":2,"security_updates":1,"system_updates_available":true,"security_updates_available":true,"system_updates_list":["one","two"],"security_updates_list":["one"]}' >/tmp/syscheck_updates.json
"$SYSCHECKS_BIN" banner --no-emojies --disk-used-threshold 100 >/tmp/syschecks-banner-pending-updates.txt
grep -q 'Number of system updates available:.*2' /tmp/syschecks-banner-pending-updates.txt
grep -q 'Number of security updates available:.*1' /tmp/syschecks-banner-pending-updates.txt

printf '%s\n' '{"system_updates":0,"security_updates":0,"system_updates_available":false,"security_updates_available":false,"system_updates_list":[],"security_updates_list":[]}' >/tmp/syscheck_updates.json
"$SYSCHECKS_BIN" cron updates --system >/dev/null
"$SYSCHECKS_BIN" banner --no-emojies --disk-used-threshold 100 >/tmp/syschecks-banner-healthy.txt
if grep -q 'Update status\|Automatic OS updates:\|No new .* updates available' /tmp/syschecks-banner-healthy.txt; then
	echo "Healthy update status should be entirely hidden" >&2
	exit 1
fi
"$SYSCHECKS_BIN" banner --no-emojies --all --disk-used-threshold 100 >/tmp/syschecks-banner-all.txt
grep -q 'Logged-in users:' /tmp/syschecks-banner-all.txt
grep -q 'Disk space' /tmp/syschecks-banner-all.txt
grep -q 'Installed kernels:' /tmp/syschecks-banner-all.txt
grep -q 'Automatic OS updates:.*Full system updates ON' /tmp/syschecks-banner-all.txt
grep -q 'No new system updates available' /tmp/syschecks-banner-all.txt
grep -q 'No new security updates available' /tmp/syschecks-banner-all.txt
grep -q 'Update cache is current' /tmp/syschecks-banner-all.txt
"$SYSCHECKS_BIN" cron updates --disable >/dev/null
rm -f /tmp/syscheck_updates.json

"$SYSCHECKS_BIN" cron kernels --disable
test ! -e /etc/cron.d/syschecks_kernel_cleanup
"$SYSCHECKS_BIN" cron autoupdate --disable
test ! -e /etc/cron.d/syschecks_autoupdate

echo "SysChecks banner/maintenance E2E passed"
