#!/usr/bin/env bash

set -euo pipefail

# Creates a self-extracting .run installer for SysChecks.
#
# The resulting single file bundles the syschecks binary, the package lock
# file, and the offline installer. Copy it to any Linux host - including
# air-gapped servers with no internet access - and run it as root:
#
#   ./syschecks-installer-amd64.run            # install / update
#   ./syschecks-installer-amd64.run --extract /tmp/sc   # inspect without installing
#   ./syschecks-installer-amd64.run --help
#
# Usage:
#   ./create-run-installer.sh [--arch amd64|arm64] [--version X.Y.Z]
#
# Environment overrides:
#   ARCH        target architecture (default: amd64)
#   BINARY      path to the syschecks binary to bundle
#               (default: bin/syschecks-linux-<arch>)
#   LOCK_FILE   path to the package lock file (default: package.lock.json)
#   INSTALLER   path to the offline installer (default: install-offline.sh)
#   OUTPUT_FILE path of the .run to create
#               (default: bin/syschecks-installer-<arch>.run)

readonly SCRIPT_NAME="$(basename "${BASH_SOURCE[0]}")"
readonly REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ARCH="${ARCH:-amd64}"
VERSION="${VERSION:-}"

# Parse arguments
while [[ $# -gt 0 ]]; do
	case "$1" in
	--arch)
		[[ -n "${2:-}" ]] || { echo "--arch requires a value" >&2; exit 1; }
		ARCH="$2"; shift 2 ;;
	--version)
		[[ -n "${2:-}" ]] || { echo "--version requires a value" >&2; exit 1; }
		VERSION="$2"; shift 2 ;;
	-h | --help)
		grep '^#' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
	*)
		echo "Unknown argument: $1" >&2; exit 1 ;;
	esac
done

BINARY="${BINARY:-${REPO_DIR}/bin/syschecks-linux-${ARCH}}"
LOCK_FILE="${LOCK_FILE:-${REPO_DIR}/package.lock.json}"
INSTALLER="${INSTALLER:-${REPO_DIR}/install-offline.sh}"
OUTPUT_FILE="${OUTPUT_FILE:-${REPO_DIR}/bin/syschecks-installer-${ARCH}.run}"

note() { printf '[%s] %s\n' "${SCRIPT_NAME}" "$*"; }
die() { printf '[%s] Error: %s\n' "${SCRIPT_NAME}" "$*" >&2; exit 1; }

[[ -f "${BINARY}" ]]    || die "Binary not found: ${BINARY} (build it first, e.g. ./build-advanced.sh cross)"
[[ -f "${LOCK_FILE}" ]] || die "Package lock not found: ${LOCK_FILE}"
[[ -f "${INSTALLER}" ]] || die "Offline installer not found: ${INSTALLER}"

# Derive version from git tag if not provided
if [[ -z "${VERSION}" ]]; then
	VERSION="$(git -C "${REPO_DIR}" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
	[[ -n "${VERSION}" ]] || VERSION="unknown"
fi
BUILD_DATE="$(date +%Y-%m-%d)"

note "Architecture: ${ARCH}"
note "Version:      ${VERSION}"
note "Binary:       ${BINARY}"

WORK_DIR="$(mktemp -d)"
cleanup() { rm -rf "${WORK_DIR}"; }
trap cleanup EXIT

PAYLOAD_DIR="${WORK_DIR}/payload"
mkdir -p "${PAYLOAD_DIR}/bin" "$(dirname "${OUTPUT_FILE}")"

# Assemble the payload
install -m 0755 "${BINARY}" "${PAYLOAD_DIR}/bin/syschecks"
install -m 0644 "${LOCK_FILE}" "${PAYLOAD_DIR}/package.lock.json"
install -m 0755 "${INSTALLER}" "${PAYLOAD_DIR}/install-offline.sh"

cat > "${PAYLOAD_DIR}/build-info.txt" <<EOF
VERSION=${VERSION}
ARCH=${ARCH}
BUILD_DATE=${BUILD_DATE}
EOF

note "Creating compressed payload archive"
ARCHIVE_PATH="${WORK_DIR}/payload.tar.gz"
(cd "${PAYLOAD_DIR}" && tar -czf "${ARCHIVE_PATH}" .)

# Build the self-extracting stub
STUB_PATH="${WORK_DIR}/stub.sh"
cat > "${STUB_PATH}" <<'STUB'
#!/usr/bin/env bash

set -euo pipefail

readonly SCRIPT_NAME="$(basename "$0")"
readonly ARCHIVE_LINE=__ARCHIVE_LINE__

usage() {
	cat <<USAGE
Usage: ./${SCRIPT_NAME} [OPTIONS]

Self-extracting SysChecks installer (bundles binary + package lock + installer).

Options:
  --extract DIR   Unpack the payload to DIR without installing it.
  -h, --help      Show this help text.

With no options, installs/updates SysChecks and must be run as root.
The install is idempotent:
  - First run:  installs binary, package lock, cron jobs, banner, Zabbix hook.
  - Next runs:  updates the binary and stages package.lock.latest.json,
                preserving your existing configuration.
USAGE
}

die() {
	printf '[%s] Error: %s\n' "${SCRIPT_NAME}" "$*" >&2
	exit 1
}

extract_payload() {
	local destination="$1"
	mkdir -p "${destination}"
	tail -n +"${ARCHIVE_LINE}" "$0" | tar -xzf - -C "${destination}"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
	usage
	exit 0
fi

if [[ "${1:-}" == "--extract" ]]; then
	[[ -n "${2:-}" ]] || die "--extract requires a destination directory"
	extract_payload "$2"
	printf '[%s] Extracted payload to %s\n' "${SCRIPT_NAME}" "$2"
	exit 0
fi

[[ "${EUID}" -eq 0 ]] || die "Run this installer as root, or use --extract to inspect it"

TMPDIR_PATH="$(mktemp -d)"
cleanup() { rm -rf "${TMPDIR_PATH}"; }
trap cleanup EXIT

extract_payload "${TMPDIR_PATH}"
bash "${TMPDIR_PATH}/install-offline.sh" "${TMPDIR_PATH}"
exit 0
STUB

ARCHIVE_LINE="$(( $(wc -l < "${STUB_PATH}") + 1 ))"
sed "s/__ARCHIVE_LINE__/${ARCHIVE_LINE}/" "${STUB_PATH}" > "${OUTPUT_FILE}"
cat "${ARCHIVE_PATH}" >> "${OUTPUT_FILE}"
chmod +x "${OUTPUT_FILE}"

note "Created self-extracting installer: ${OUTPUT_FILE}"
note "Size: $(du -h "${OUTPUT_FILE}" | cut -f1)"
