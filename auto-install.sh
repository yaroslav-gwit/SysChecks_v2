#!/usr/bin/env bash

# SysChecks Auto-Installation Script
# Automatically downloads and installs the latest release

set -e

# Configuration
REPO="yaroslav-gwit/SysChecks_v2"
INSTALL_DIR="/opt/syschecks"
BIN_LINK="/usr/bin/syschecks"
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

print_info() {
    echo -e "${BLUE}$1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

get_latest_version() {
    print_info "Getting latest version..."
    local version
    version=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"v?([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
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

install_binary() {
    local version="$1"
    local is_update="$2"
    
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
        local lock_url="https://github.com/$REPO/releases/download/v$version/$PACKAGE_LOCK_NAME"
        local lock_path="$INSTALL_DIR/package.lock.latest.json"
        download_file "$lock_url" "$lock_path" "latest package lock (as package.lock.latest.json)"
        chmod 0644 "$lock_path"
        chown root:root "$lock_path"
        print_success "Latest package lock saved to $lock_path"
        print_warning "Your existing package.lock.json was preserved"
        print_info "Review package.lock.latest.json and merge changes if needed"
    else
        # For new installations, download as package.lock.json
        local lock_url="https://github.com/$REPO/releases/download/v$version/$PACKAGE_LOCK_NAME"
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

verify_installation() {
    print_info "Verifying installation..."
    
    if command -v syschecks &> /dev/null; then
        local version
        version=$(syschecks version 2>/dev/null || echo "unknown")
        print_success "syschecks is installed and working"
        print_info "Installed version: $version"
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
    echo -e "${BLUE}Usage examples:${NC}"
    echo "  syschecks version              # Show version"
    echo "  syschecks kernel               # Check kernel reboot status"
    echo "  syschecks kernel cleanup       # Clean up old kernels"
    echo "  syschecks updates              # Check for updates"
    echo "  syschecks banner               # Display system banner"
    echo "  syschecks --help               # Show all commands"
    echo ""
    echo -e "${BLUE}To display banner on SSH login, run:${NC}"
    echo "  echo '([ -z \"\$PS1\" ] && true) || syschecks banner' >> /etc/profile.d/syschecks_banner.sh && chmod 0755 /etc/profile.d/syschecks_banner.sh"
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
else
    print_info "Skipping cron setup (update mode - preserving existing configuration)"
fi

# Verify installation
echo ""
verify_installation

# Show usage information
show_usage
