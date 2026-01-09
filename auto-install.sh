#!/usr/bin/env bash

# SysChecks Auto-Installation Script
# Automatically downloads and installs the latest release

set -e

# Configuration
REPO="yaroslav-gwit/SysChecks_v2"
INSTALL_DIR="/opt/syschecks"
BIN_LINK="/bin/syschecks"
BINARY_NAME="syschecks"
REMOTE_BINARY_NAME="syschecks-linux-amd64"
PACKAGE_LOCK_NAME="package.lock.json"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check if running as root
if [[ ${EUID} != 0 ]]; then
    echo -e "${RED}Please run this script as root!${NC}"
    exit 1
fi

# Check for required tools
if ! command -v curl &> /dev/null; then
    echo -e "${RED}curl is required but not installed. Please install it first.${NC}"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo -e "${RED}jq is required but not installed.${NC}"
    echo -e "${YELLOW}Install it with:${NC}"
    echo "  Ubuntu/Debian: sudo apt install jq"
    echo "  RHEL/CentOS:   sudo yum install jq"
    echo "  Alpine:        sudo apk add jq"
    exit 1
fi

print_info() {
    echo -e "${BLUE}$1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}" >&2
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}" >&2
}

print_error() {
    echo -e "${RED}✗ $1${NC}" >&2
}

get_latest_version() {
    print_info "Getting latest version..."
    local version
    version=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | \
        jq -r '.tag_name' | \
        sed 's/^v//')
    
    if [ -z "$version" ] || [ "$version" = "null" ]; then
        print_error "Failed to get latest version from GitHub API"
        exit 1
    fi
    
    echo "$version"
}

check_existing_installation() {
    if [ -f "$INSTALL_DIR/$PACKAGE_LOCK_NAME" ]; then
        print_warning "Existing installation detected at $INSTALL_DIR"
        print_info "Will update binary only and preserve your configuration"
        return 0  # Existing installation
    else
        return 1  # New installation
    fi
}

download_file() {
    local url="$1"
    local output="$2"
    local description="$3"
    
    print_info "Downloading $description..."
    if ! curl -fsSL "$url" -o "$output"; then
        print_error "Failed to download $description"
        print_error "URL: $url"
        exit 1
    fi
    print_success "Downloaded $description"
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
    local version="$1"
    local is_update="$2"
    
    # Clean up old binaries first
    cleanup_old_binaries
    
    # Create installation directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        print_info "Creating directory $INSTALL_DIR..."
        mkdir -p "$INSTALL_DIR"
    fi
    
    # Download the binary
    local binary_url="https://github.com/$REPO/releases/download/v$version/$REMOTE_BINARY_NAME"
    local temp_binary="/tmp/$BINARY_NAME.$$"
    download_file "$binary_url" "$temp_binary" "syschecks binary v$version"
    
    # Install binary to /opt/syschecks
    print_info "Installing binary to $INSTALL_DIR/$BINARY_NAME..."
    cp "$temp_binary" "$INSTALL_DIR/$BINARY_NAME"
    chmod 0755 "$INSTALL_DIR/$BINARY_NAME"
    chown root:root "$INSTALL_DIR/$BINARY_NAME"
    rm -f "$temp_binary"
    print_success "Binary installed to $INSTALL_DIR/$BINARY_NAME"
    
    # Create symlink or copy to /usr/bin
    print_info "Linking binary to $BIN_LINK..."
    if ln -sf "$INSTALL_DIR/$BINARY_NAME" "$BIN_LINK" 2>/dev/null; then
        print_success "Symlink created: $BIN_LINK -> $INSTALL_DIR/$BINARY_NAME"
    else
        print_warning "Symlink failed, copying binary instead..."
        cp "$INSTALL_DIR/$BINARY_NAME" "$BIN_LINK"
        chmod 0755 "$BIN_LINK"
        chown root:root "$BIN_LINK"
        print_success "Binary copied to $BIN_LINK"
    fi
}

install_package_lock() {
    local version="$1"
    local is_update="$2"
    
    if [ "$is_update" = "true" ]; then
        # For updates, download as package.lock.latest.json
        local lock_url="https://raw.githubusercontent.com/$REPO/refs/tags/v$version/$PACKAGE_LOCK_NAME"
        local lock_path="$INSTALL_DIR/package.lock.latest.json"
        download_file "$lock_url" "$lock_path" "latest package lock (as package.lock.latest.json)"
        chmod 0644 "$lock_path"
        chown root:root "$lock_path"
        print_success "Latest package lock saved to $lock_path"
        print_warning "Your existing package.lock.json was preserved"
        print_info "Review package.lock.latest.json and merge changes if needed"
    else
        # For new installations, download as package.lock.json
        local lock_url="https://raw.githubusercontent.com/$REPO/refs/tags/v$version/$PACKAGE_LOCK_NAME"
        local lock_path="$INSTALL_DIR/$PACKAGE_LOCK_NAME"
        download_file "$lock_url" "$lock_path" "package lock file"
        chmod 0644 "$lock_path"
        chown root:root "$lock_path"
        print_success "Package lock installed to $lock_path"
    fi
}

enable_bash_completion() {
    # Check if bash-completion is available
    if [ -d "/etc/bash_completion.d" ] || [ -d "/usr/share/bash-completion/completions" ]; then
        print_info "Enabling bash completion..."
        
        # Try to generate completion
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
    
    # Set up cache update cron
    if command -v syschecks &> /dev/null; then
        syschecks cron init 2>/dev/null || print_warning "Could not set up cache cron job"
        
        # Enable automatic security updates
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
    
    # Check if banner integration already exists somewhere in /etc/profile or profile.d
    if grep -rq "syschecks banner" /etc/profile /etc/profile.d/ 2>/dev/null; then
        print_warning "Banner integration already exists (skipping)"
        print_info "Found in: $(grep -rl 'syschecks banner' /etc/profile /etc/profile.d/ 2>/dev/null | head -1)"
        return 0
    fi
    
    # Create the banner script with proper checks
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
    # SSH session - show full banner with emojis
    syschecks banner 2>/dev/null
else
    # Local console (tty) - show simplified version without emojis
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
    
    # Check for Zabbix config files (locations supported by syschecks)
    if [ -f "/etc/zabbix/zabbix_agentd.conf" ] || [ -f "/etc/zabbix_agentd.conf" ]; then
        print_info "Zabbix configuration found, enabling integration..."
        
        if command -v syschecks &> /dev/null; then
            if syschecks zabbix init 2>/dev/null; then
                print_success "Zabbix integration activated"
                
                # Attempt to restart Zabbix agent to apply changes
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
    
    # Clear shell command cache
    hash -r 2>/dev/null || true
    
    # Show which binary is being used
    local binary_path
    binary_path=$(which syschecks 2>/dev/null || echo "not found")
    
    if [ "$binary_path" != "$BIN_LINK" ] && [ "$binary_path" != "not found" ]; then
        print_warning "Found syschecks at unexpected location: $binary_path"
        print_info "Expected location: $BIN_LINK"
    fi
    
    if command -v syschecks &> /dev/null; then
        local version
        version=$(syschecks version 2>/dev/null || echo "unknown")
        print_success "syschecks is installed and working"
        print_info "Installed version: $version"
        print_info "Binary location: $binary_path"
        
        # Recommend shell restart if needed
        if [ "$binary_path" != "$BIN_LINK" ]; then
            print_warning "You may need to restart your shell or run: hash -r"
        fi
        return 0
    else
        print_error "Installation verification failed"
        print_error "syschecks is not in PATH"
        exit 1
    fi
}

show_usage() {
    echo ""
    echo -e "${GREEN}Installation completed successfully! 🎉${NC}"
    echo ""
    echo -e "${YELLOW}Important:${NC} If you're updating, restart your shell or run: ${BLUE}hash -r${NC}"
    echo ""
    echo -e "${BLUE}Usage examples:${NC}"
    echo "  syschecks version              # Show version"
    echo "  syschecks kernel               # Check kernel reboot status"
    echo "  syschecks kernel cleanup       # Clean up old kernels"
    echo "  syschecks updates              # Check for updates"
    echo "  syschecks banner               # Display system banner"
    echo "  syschecks --help               # Show all commands"
    echo ""
    echo -e "${BLUE}Login banner:${NC}"
    echo "  The system banner is automatically displayed on SSH login."
    echo "  Config: /etc/profile.d/syschecks_banner.sh"
    echo ""
}

# Main execution
echo -e "${GREEN}╔═══════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  SysChecks Auto-Installation Script      ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════╝${NC}"
echo ""

# Check for existing installation
IS_UPDATE="false"
if check_existing_installation; then
    IS_UPDATE="true"
fi

# Get latest version
VERSION=$(get_latest_version)
print_info "Latest version: v$VERSION"
echo ""

# Install binary
install_binary "$VERSION" "$IS_UPDATE"

# Install or update package lock
install_package_lock "$VERSION" "$IS_UPDATE"

# Set correct ownership and permissions for install directory
chown -R root:root "$INSTALL_DIR"
chmod 0755 "$INSTALL_DIR"

# Enable bash completion
enable_bash_completion

# Only set up cron jobs on new installations
if [ "$IS_UPDATE" = "false" ]; then
    setup_cron_jobs
    create_initial_cache
    setup_login_banner
    setup_zabbix_integration
else
    print_info "Skipping cron setup (update mode - preserving existing configuration)"
fi

# Verify installation
echo ""
verify_installation

# Show usage information
show_usage
