#!/usr/bin/env bash

# SysChecks Offline Installer
#
# Installs SysChecks from local files only - no internet access required.
# This script is meant to be run from inside the extracted payload of a
# self-extracting "syschecks-installer.run" bundle (see create-run-installer.sh),
# but it can also be run by hand by pointing it at a directory that contains:
#
#   bin/syschecks         the syschecks binary
#   package.lock.json     the default package lock file
#   build-info.txt        (optional) VERSION / BUILD_DATE metadata
#
# Usage:
#   ./install-offline.sh [PAYLOAD_DIR]
#
# PAYLOAD_DIR defaults to the directory this script lives in.

set -e

# Configuration
INSTALL_DIR="/opt/syschecks"
BIN_LINK="/bin/syschecks"
BINARY_NAME="syschecks"
PACKAGE_LOCK_NAME="package.lock.json"

# Resolve the payload directory (where the binary + lock file live)
PAYLOAD_DIR="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
SRC_BINARY="$PAYLOAD_DIR/bin/$BINARY_NAME"
SRC_LOCK="$PAYLOAD_DIR/$PACKAGE_LOCK_NAME"
BUILD_INFO="$PAYLOAD_DIR/build-info.txt"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_info()    { echo -e "${BLUE}$1${NC}" >&2; }
print_success() { echo -e "${GREEN}✓ $1${NC}" >&2; }
print_warning() { echo -e "${YELLOW}⚠ $1${NC}" >&2; }
print_error()   { echo -e "${RED}✗ $1${NC}" >&2; }

# Check if running as root
if [[ ${EUID} != 0 ]]; then
    print_error "Please run this installer as root!"
    exit 1
fi

# Validate payload
if [ ! -f "$SRC_BINARY" ]; then
    print_error "Binary not found in payload: $SRC_BINARY"
    exit 1
fi
if [ ! -f "$SRC_LOCK" ]; then
    print_error "Package lock not found in payload: $SRC_LOCK"
    exit 1
fi

# Read embedded version metadata if present
VERSION="unknown"
if [ -f "$BUILD_INFO" ]; then
    # shellcheck disable=SC1090
    . "$BUILD_INFO" 2>/dev/null || true
    [ -n "${VERSION:-}" ] || VERSION="unknown"
fi

check_existing_installation() {
    if [ -f "$INSTALL_DIR/$PACKAGE_LOCK_NAME" ]; then
        print_warning "Existing installation detected at $INSTALL_DIR"
        print_info "Will update binary only and preserve your configuration"
        return 0  # Existing installation
    else
        return 1  # New installation
    fi
}

cleanup_old_binaries() {
    print_info "Cleaning up old binary locations..."
    local locations=(
        "/usr/bin/syschecks"
        "/usr/local/bin/syschecks"
        "/bin/syschecks"
    )

    for location in "${locations[@]}"; do
        if [ -f "$location" ] || [ -L "$location" ]; then
            rm -f "$location"
            print_success "Removed old binary: $location"
        fi
    done
}

install_binary() {
    cleanup_old_binaries

    if [ ! -d "$INSTALL_DIR" ]; then
        print_info "Creating directory $INSTALL_DIR..."
        mkdir -p "$INSTALL_DIR"
    fi

    print_info "Installing binary to $INSTALL_DIR/$BINARY_NAME..."
    install -m 0755 -o root -g root "$SRC_BINARY" "$INSTALL_DIR/$BINARY_NAME"
    print_success "Binary installed to $INSTALL_DIR/$BINARY_NAME"

    print_info "Linking binary to $BIN_LINK..."
    if ln -sf "$INSTALL_DIR/$BINARY_NAME" "$BIN_LINK" 2>/dev/null; then
        print_success "Symlink created: $BIN_LINK -> $INSTALL_DIR/$BINARY_NAME"
    else
        print_warning "Symlink failed, copying binary instead..."
        install -m 0755 -o root -g root "$INSTALL_DIR/$BINARY_NAME" "$BIN_LINK"
        print_success "Binary copied to $BIN_LINK"
    fi
}

install_package_lock() {
    local is_update="$1"

    if [ "$is_update" = "true" ]; then
        # Preserve the admin's existing lock; stage the new one alongside it
        local lock_path="$INSTALL_DIR/package.lock.latest.json"
        install -m 0644 -o root -g root "$SRC_LOCK" "$lock_path"
        print_success "Latest package lock saved to $lock_path"
        print_warning "Your existing package.lock.json was preserved"
        print_info "Review package.lock.latest.json and merge changes if needed"
    else
        local lock_path="$INSTALL_DIR/$PACKAGE_LOCK_NAME"
        install -m 0644 -o root -g root "$SRC_LOCK" "$lock_path"
        print_success "Package lock installed to $lock_path"
    fi
}

enable_bash_completion() {
    if [ -d "/etc/bash_completion.d" ] || [ -d "/usr/share/bash-completion/completions" ]; then
        print_info "Enabling bash completion..."
        if command -v syschecks &> /dev/null; then
            if syschecks completion bash > /etc/bash_completion.d/syschecks 2>/dev/null; then
                chmod 0644 /etc/bash_completion.d/syschecks
                print_success "Bash completion enabled"
            else
                print_warning "Could not generate bash completion (command may not support it yet)"
            fi
        else
            print_warning "Could not enable bash completion (syschecks not in PATH yet)"
        fi
    else
        print_info "Bash completion not available on this system (skipping)"
    fi
}

setup_cron_jobs() {
    print_info "Setting up cron jobs..."

    # Remove deprecated cron jobs
    rm -f /etc/cron.d/automatic_system_updates_hold 2>/dev/null || true
    rm -f /etc/cron.d/automatic_security_updates 2>/dev/null || true
    rm -f /etc/cron.d/automatic_system_updates 2>/dev/null || true
    rm -f /etc/cron.d/syschecks 2>/dev/null || true

    if command -v syschecks &> /dev/null; then
        syschecks cron init 2>/dev/null || print_warning "Could not set up cache cron job"
        syschecks cron updates --security 2>/dev/null || print_warning "Could not set up security updates cron"
        print_success "Cron jobs configured"
    else
        print_warning "Syschecks not available yet, skipping cron setup"
    fi
}

create_initial_cache() {
    print_info "Creating initial update cache..."
    if command -v syschecks &> /dev/null; then
        if syschecks updates --cache-create 2>/dev/null; then
            print_success "Initial cache created"
        else
            print_warning "Could not create initial cache (this is normal on first install)"
        fi
    fi
}

setup_login_banner() {
    print_info "Setting up SSH login banner..."

    local banner_script="/etc/profile.d/syschecks_banner.sh"

    if grep -rq "syschecks banner" /etc/profile /etc/profile.d/ 2>/dev/null; then
        print_warning "Banner integration already exists (skipping)"
        print_info "Found in: $(grep -rl 'syschecks banner' /etc/profile /etc/profile.d/ 2>/dev/null | head -1)"
        return 0
    fi

    cat > "$banner_script" << 'BANNER_EOF'
#!/bin/bash
# SysChecks Login Banner
# Shows system information on interactive SSH login

# Only run for interactive sessions
[[ $- != *i* ]] && return

# Check if syschecks is available
command -v syschecks &>/dev/null || return

# Detect if this is an SSH session or local console
if [[ -n "$SSH_CLIENT" ]] || [[ -n "$SSH_TTY" ]] || [[ -n "$SSH_CONNECTION" ]]; then
    syschecks banner 2>/dev/null
else
    syschecks banner --no-emojies 2>/dev/null
fi
BANNER_EOF

    chmod 0755 "$banner_script"
    chown root:root "$banner_script"

    print_success "Login banner configured at $banner_script"
    print_info "Banner will show on next SSH login"
}

setup_zabbix_integration() {
    print_info "Checking for Zabbix Agent..."

    if [ -f "/etc/zabbix/zabbix_agentd.conf" ] || [ -f "/etc/zabbix_agentd.conf" ]; then
        print_info "Zabbix configuration found, enabling integration..."
        if command -v syschecks &> /dev/null; then
            if syschecks zabbix init 2>/dev/null; then
                print_success "Zabbix integration activated"
                if command -v systemctl &> /dev/null; then
                    print_info "Restarting Zabbix agent..."
                    systemctl try-restart zabbix-agent zabbix-agent2 2>/dev/null || true
                fi
            else
                print_warning "Failed to activate Zabbix integration"
            fi
        fi
    else
        print_info "Zabbix agent config not found (skipping integration)"
    fi
}

verify_installation() {
    print_info "Verifying installation..."
    hash -r 2>/dev/null || true

    local binary_path
    binary_path=$(which syschecks 2>/dev/null || echo "not found")

    if [ "$binary_path" != "$BIN_LINK" ] && [ "$binary_path" != "not found" ]; then
        print_warning "Found syschecks at unexpected location: $binary_path"
        print_info "Expected location: $BIN_LINK"
    fi

    if command -v syschecks &> /dev/null; then
        local installed_version
        installed_version=$(syschecks version 2>/dev/null || echo "unknown")
        print_success "syschecks is installed and working"
        print_info "Installed version: $installed_version"
        print_info "Binary location: $binary_path"
        return 0
    else
        print_error "Installation verification failed"
        print_error "syschecks is not in PATH"
        exit 1
    fi
}

show_usage() {
    echo ""
    echo -e "${GREEN}Offline installation completed successfully! 🎉${NC}"
    echo ""
    echo -e "${YELLOW}Important:${NC} If you're updating, restart your shell or run: ${BLUE}hash -r${NC}"
    echo ""
    echo -e "${BLUE}Usage examples:${NC}"
    echo "  syschecks version              # Show version"
    echo "  syschecks kernel               # Check kernel reboot status"
    echo "  syschecks updates              # Check for updates"
    echo "  syschecks banner               # Display system banner"
    echo "  syschecks --help               # Show all commands"
    echo ""
}

# Main execution
echo -e "${GREEN}╔═══════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  SysChecks Offline Installer              ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════╝${NC}"
echo ""
print_info "Bundled version: v$VERSION"
echo ""

IS_UPDATE="false"
if check_existing_installation; then
    IS_UPDATE="true"
fi

install_binary
install_package_lock "$IS_UPDATE"

chown -R root:root "$INSTALL_DIR"
chmod 0755 "$INSTALL_DIR"

enable_bash_completion

if [ "$IS_UPDATE" = "false" ]; then
    setup_cron_jobs
    create_initial_cache
    setup_login_banner
    setup_zabbix_integration
else
    print_info "Skipping cron setup (update mode - preserving existing configuration)"
fi

echo ""
verify_installation
show_usage
