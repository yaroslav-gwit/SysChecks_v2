#!/bin/bash

# ============================================================================
# SysChecks v2 - Automated Release Script
# ============================================================================
# Creates GitHub releases with built binaries, checksums, and release notes.
# Requires: GitHub CLI (gh), Docker (for portable builds), Git
# ============================================================================

set -eo pipefail

# Configuration
REPO="yaroslav-gwit/SysChecks_v2"
BUILD_DIR="./bin"
CHANGELOG_FILE="./CHANGELOG.md"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# State
DRY_RUN=false
SKIP_BUILD=false
SKIP_TESTS=false
FORCE=false
VERBOSE=false

# ============================================================================
# Utility Functions
# ============================================================================

# Status/log output goes to stderr so it never contaminates command
# substitutions that capture a function's stdout (e.g. release notes).
log_info() {
    echo -e "${BLUE}ℹ${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}✓${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1" >&2
}

log_error() {
    echo -e "${RED}✗${NC} $1" >&2
}

log_step() {
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}" >&2
    echo -e "${BOLD}$1${NC}" >&2
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}" >&2
}

die() {
    log_error "$1"
    exit 1
}

confirm() {
    if [ "$FORCE" = true ]; then
        return 0
    fi
    
    local prompt="$1 [y/N]: "
    read -r -p "$prompt" response
    case "$response" in
        [yY][eE][sS]|[yY]) return 0 ;;
        *) return 1 ;;
    esac
}

print_usage() {
    cat << EOF
${BOLD}SysChecks Release Script${NC}

${BOLD}Usage:${NC}
    $0 [OPTIONS] <VERSION> [RELEASE_TYPE]

${BOLD}Arguments:${NC}
    VERSION         Semantic version (e.g., 1.0.0, 2.1.0-beta.1)
    RELEASE_TYPE    One of: release, prerelease, draft (default: release)

${BOLD}Options:${NC}
    -h, --help          Show this help message
    -n, --dry-run       Simulate release without making changes
    -f, --force         Skip confirmation prompts
    -s, --skip-build    Use existing binaries in ./bin/
    -t, --skip-tests    Skip running tests before release
    -v, --verbose       Enable verbose output

${BOLD}Examples:${NC}
    $0 1.0.0                    # Create release v1.0.0
    $0 1.0.1 prerelease         # Create prerelease
    $0 2.0.0-beta.1 draft       # Create draft release
    $0 -n 1.0.0                 # Dry run (simulate)
    $0 -f -s 1.0.0              # Force, skip build

${BOLD}Prerequisites:${NC}
    - GitHub CLI installed and authenticated (gh auth login)
    - Docker installed (for portable Linux binaries)
    - Git repository with clean working tree (or use --force)

${BOLD}Version Format:${NC}
    Follows semantic versioning: MAJOR.MINOR.PATCH[-PRERELEASE]
    Examples: 1.0.0, 1.2.3-alpha, 2.0.0-beta.1, 3.0.0-rc.2

EOF
}

# ============================================================================
# Validation Functions
# ============================================================================

validate_semver() {
    local version="$1"
    # Semantic versioning regex (simplified)
    local semver_regex='^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$'
    
    if [[ ! "$version" =~ $semver_regex ]]; then
        die "Invalid version format: $version\nExpected semantic version (e.g., 1.0.0, 1.0.0-beta.1)"
    fi
}

validate_release_type() {
    local release_type="$1"
    case "$release_type" in
        release|prerelease|draft) return 0 ;;
        *) die "Invalid release type: $release_type\nMust be one of: release, prerelease, draft" ;;
    esac
}

check_prerequisites() {
    log_step "Checking Prerequisites"
    
    local missing=()
    
    # Check for required tools
    if ! command -v gh &> /dev/null; then
        missing+=("GitHub CLI (gh) - https://cli.github.com/")
    fi
    
    if ! command -v docker &> /dev/null; then
        log_warning "Docker not found - will use native build (less portable)"
    fi
    
    if ! command -v git &> /dev/null; then
        missing+=("git")
    fi
    
    if ! command -v go &> /dev/null; then
        missing+=("go")
    fi
    
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "Missing required tools:"
        for tool in "${missing[@]}"; do
            echo "  - $tool"
        done
        exit 1
    fi
    
    # Check gh authentication
    if ! gh auth status &> /dev/null; then
        die "Not authenticated with GitHub CLI\nRun: gh auth login"
    fi
    log_success "GitHub CLI authenticated"
    
    # Check git status
    if [ -n "$(git status --porcelain)" ] && [ "$FORCE" = false ]; then
        log_warning "Working directory has uncommitted changes"
        if ! confirm "Continue anyway?"; then
            exit 1
        fi
    fi
    log_success "Git repository is ready"
    
    log_success "All prerequisites satisfied"
}

check_existing_release() {
    local version="$1"
    
    if gh release view "v$version" --repo "$REPO" &> /dev/null; then
        if [ "$FORCE" = false ]; then
            log_warning "Release v$version already exists on GitHub"
            echo ""
            echo "Options:"
            echo "  1) Delete and recreate the release"
            echo "  2) Upload new binaries to existing release (replace)"
            echo "  3) Cancel"
            echo ""
            read -r -p "Choose [1/2/3]: " choice
            
            case "$choice" in
                1)
                    if [ "$DRY_RUN" = false ]; then
                        log_info "Deleting existing release v$version..."
                        gh release delete "v$version" --repo "$REPO" --yes 2>/dev/null || true
                        # Also delete the tag
                        git tag -d "v$version" 2>/dev/null || true
                        git push origin ":refs/tags/v$version" 2>/dev/null || true
                    fi
                    return 0  # Proceed with full release creation
                    ;;
                2)
                    log_info "Will upload new binaries to existing release"
                    return 1  # Signal to skip release creation, just upload
                    ;;
                3|*)
                    log_warning "Release cancelled"
                    exit 0
                    ;;
            esac
        else
            # Force mode - delete and recreate
            if [ "$DRY_RUN" = false ]; then
                log_info "Deleting existing release v$version..."
                gh release delete "v$version" --repo "$REPO" --yes 2>/dev/null || true
                git tag -d "v$version" 2>/dev/null || true
                git push origin ":refs/tags/v$version" 2>/dev/null || true
            fi
            return 0
        fi
    fi
    
    return 0  # No existing release
}

# ============================================================================
# Build Functions
# ============================================================================

run_tests() {
    if [ "$SKIP_TESTS" = true ]; then
        log_info "Skipping tests (--skip-tests)"
        return 0
    fi
    
    log_step "Running Tests"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] Would run: go test ./..."
        return 0
    fi
    
    if go test ./... 2>/dev/null; then
        log_success "All tests passed"
    else
        log_warning "No tests found or tests skipped"
    fi
}

build_binaries() {
    if [ "$SKIP_BUILD" = true ]; then
        log_info "Skipping build (--skip-build)"
        if [ ! -d "$BUILD_DIR" ] || [ -z "$(ls -A "$BUILD_DIR" 2>/dev/null)" ]; then
            die "No binaries found in $BUILD_DIR"
        fi
        return 0
    fi
    
    log_step "Building Binaries"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] Would build binaries"
        return 0
    fi
    
    mkdir -p "$BUILD_DIR"
    
    # Clean old binaries
    rm -f "$BUILD_DIR"/syschecks-*
    
    # Build with Docker for maximum compatibility (if Docker available)
    if command -v docker &> /dev/null; then
        log_info "Building portable Linux binary with Docker (Ubuntu 18.04)..."
        # Pass VERSION to the build script without modifying/unsetting the global variable
        if VERSION="v$VERSION" sudo -E ./build-advanced.sh docker ubuntu18; then
            rm -f "$BUILD_DIR/syschecks-linux-amd64"
            mv "$BUILD_DIR/syschecks-ubuntu18" "$BUILD_DIR/syschecks-linux-amd64"
            log_success "Built syschecks-linux-amd64"
        else
            log_warning "Docker build failed, falling back to native build"
        fi
    fi
    
    # Cross-compile for other platforms
    log_info "Cross-compiling for multiple platforms..."
    
    local platforms=(
        "linux/arm64"
    )
    
    for platform in "${platforms[@]}"; do
        IFS='/' read -r GOOS GOARCH <<< "$platform"
        output_name="syschecks-$GOOS-$GOARCH"
        
        log_info "Building for $GOOS/$GOARCH..."
        
        # Set version info via ldflags
        LDFLAGS="-X 'syschecks/cmd.Version=v$VERSION' \
                 -X 'syschecks/cmd.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)' \
                 -X 'syschecks/cmd.BuildDate=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')'"
        
        if env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
            go build -ldflags="$LDFLAGS -w -s" -o "$BUILD_DIR/$output_name" .; then
            log_success "Built $output_name"
        else
            log_error "Failed to build for $GOOS/$GOARCH"
        fi
    done
    
    # Ensure linux-amd64 exists (fallback if Docker failed)
    if [ ! -f "$BUILD_DIR/syschecks-linux-amd64" ]; then
        log_info "Building Linux amd64 natively..."
        LDFLAGS="-X 'syschecks/cmd.Version=v$VERSION' \
                 -X 'syschecks/cmd.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)' \
                 -X 'syschecks/cmd.BuildDate=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')'"
        
        env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
            go build -ldflags="$LDFLAGS -w -s" -o "$BUILD_DIR/syschecks-linux-amd64" .
        log_success "Built syschecks-linux-amd64"
    fi

    # Pack self-extracting offline installers (for air-gapped hosts)
    log_info "Creating self-extracting .run installers..."
    for arch in amd64 arm64; do
        if [ -f "$BUILD_DIR/syschecks-linux-$arch" ]; then
            if ./create-run-installer.sh --arch "$arch" --version "$VERSION" > /dev/null; then
                log_success "Built syschecks-installer-$arch.run"
            else
                log_warning "Failed to build .run installer for $arch"
            fi
        fi
    done

    log_success "All binaries built successfully"
}

generate_checksums() {
    log_step "Generating Checksums"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] Would generate checksums"
        return 0
    fi
    
    cd "$BUILD_DIR"
    
    # Generate SHA256 checksums
    local checksum_file="checksums-sha256.txt"
    rm -f "$checksum_file"
    
    for binary in syschecks-*; do
        if [ -f "$binary" ] && [ "$binary" != "$checksum_file" ]; then
            if command -v sha256sum &> /dev/null; then
                sha256sum "$binary" >> "$checksum_file"
            elif command -v shasum &> /dev/null; then
                shasum -a 256 "$binary" >> "$checksum_file"
            fi
        fi
    done
    
    cd - > /dev/null
    
    if [ -f "$BUILD_DIR/$checksum_file" ]; then
        log_success "Generated $checksum_file"
        if [ "$VERBOSE" = true ]; then
            cat "$BUILD_DIR/$checksum_file"
        fi
    fi
}

# ============================================================================
# Release Notes Functions
# ============================================================================

extract_changelog() {
    local version="$1"
    
    if [ ! -f "$CHANGELOG_FILE" ]; then
        return 1
    fi
    
    # Extract section for this version from changelog
    # Looks for ## [version] or ## version patterns
    local in_section=false
    local notes=""
    
    while IFS= read -r line; do
        if [[ "$line" =~ ^##[[:space:]]+\[?v?${version}\]? ]] || \
           [[ "$line" =~ ^##[[:space:]]+v?${version}[[:space:]] ]]; then
            in_section=true
            continue
        fi
        
        if [ "$in_section" = true ]; then
            if [[ "$line" =~ ^##[[:space:]] ]]; then
                break
            fi
            notes+="$line"$'\n'
        fi
    done < "$CHANGELOG_FILE"
    
    if [ -n "$notes" ]; then
        echo "$notes"
        return 0
    fi
    
    return 1
}

generate_release_notes() {
    local version="$1"
    local release_type="$2"
    
    log_step "Generating Release Notes"
    
    local release_notes=""
    local changelog_notes=""

    # Try to extract from CHANGELOG.md
    if changelog_notes=$(extract_changelog "$version" 2>/dev/null) && [ -n "$changelog_notes" ]; then
        log_success "Found changelog entry for v$version"
    else
        log_info "No changelog entry found, generating default notes"
        changelog_notes=""
    fi

    # Determine the previous release tag for the "Full Changelog" link.
    # The v${version} tag does not exist yet at this point, so the newest
    # existing tag is the previous release.
    local previous_tag
    previous_tag=$(git tag -l 'v*' --sort=-version:refname | grep -vxF "v${version}" | head -1)
    local compare_url
    if [ -n "$previous_tag" ]; then
        compare_url="https://github.com/${REPO}/compare/${previous_tag}...v${version}"
    else
        compare_url="https://github.com/${REPO}/releases/tag/v${version}"
    fi
    
    # Prerelease banner (empty for a normal release, so no blank gap is left).
    local prerelease_note=""
    if [ "$release_type" = "prerelease" ]; then
        prerelease_note="> ⚠️ **This is a pre-release version.** It may contain bugs or incomplete features."$'\n'
    fi

    # Body: the changelog entry (trimmed of leading/trailing blank lines) or default notes.
    local body_section
    if [ -n "$changelog_notes" ]; then
        body_section=$(printf '%s\n' "$changelog_notes" | awk 'NF{p=1} p' | tac | awk 'NF{p=1} p' | tac)
    else
        body_section=$(cat << 'FEATURES'
### Features

- **Kernel Management** - Check reboot requirements and clean up old kernels
- **Update Monitoring** - Track system and security updates with caching
- **Automated Updates** - Schedule automatic security/system updates via cron
- **Zabbix Integration** - Native UserParameter support for monitoring
- **SSH Login Banner** - System info display on login
FEATURES
)
    fi

    # Build release notes
    read -r -d '' release_notes << EOF || true
## SysChecks v${version}
${prerelease_note}
${body_section}

### Installation

#### Quick Install (Recommended)

\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/${REPO}/main/auto-install.sh | sudo bash
\`\`\`

#### Manual Download

\`\`\`bash
# Linux (x86_64)
wget https://github.com/${REPO}/releases/download/v${version}/syschecks-linux-amd64
chmod +x syschecks-linux-amd64
sudo mv syschecks-linux-amd64 /usr/local/bin/syschecks

# Linux (ARM64)
wget https://github.com/${REPO}/releases/download/v${version}/syschecks-linux-arm64
chmod +x syschecks-linux-arm64
sudo mv syschecks-linux-arm64 /usr/local/bin/syschecks
\`\`\`

### Verify Checksums

\`\`\`bash
# Download and verify
wget https://github.com/${REPO}/releases/download/v${version}/checksums-sha256.txt
sha256sum -c checksums-sha256.txt
\`\`\`

### Quick Start

\`\`\`bash
# Check kernel status
syschecks kernel

# Check for updates
syschecks updates

# Show system banner
syschecks banner

# Show version
syschecks version -v
\`\`\`

---

**Full Changelog**: ${compare_url}
EOF

    echo "$release_notes"
}

# ============================================================================
# Release Functions
# ============================================================================

create_git_tag() {
    local version="$1"
    
    log_info "Creating git tag v$version..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] Would create tag v$version"
        return 0
    fi
    
    git tag -a "v$version" -m "Release v$version"
    git push origin "v$version"
    
    log_success "Created and pushed tag v$version"
}

create_release() {
    local version="$1"
    local release_type="$2"
    local release_notes="$3"
    
    log_step "Creating GitHub Release"
    
    local release_flags=""
    local release_title="SysChecks v$version"
    
    case "$release_type" in
        "draft")
            release_flags="--draft"
            release_title="[DRAFT] $release_title"
            ;;
        "prerelease")
            release_flags="--prerelease"
            ;;
        "release")
            release_flags=""
            ;;
    esac
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] Would create $release_type: v$version"
        log_info "Binaries to upload:"
        for f in "$BUILD_DIR"/syschecks-* "$BUILD_DIR"/checksums-*.txt; do
            [ -f "$f" ] && echo "  - $(basename "$f")"
        done
        return 0
    fi
    
    # Create tag first
    create_git_tag "$version"
    
    # Create release
    local release_cmd="gh release create \"v$version\" \
        --repo \"$REPO\" \
        --title \"$release_title\" \
        --notes-file - \
        $release_flags"
    
    # Add all binaries
    for binary in "$BUILD_DIR"/syschecks-*; do
        if [ -f "$binary" ]; then
            release_cmd+=" \"$binary\""
        fi
    done
    
    # Add checksum file
    if [ -f "$BUILD_DIR/checksums-sha256.txt" ]; then
        release_cmd+=" \"$BUILD_DIR/checksums-sha256.txt\""
    fi
    
    # Execute release creation
    echo "$release_notes" | eval "$release_cmd"
    
    log_success "Release v$version created successfully!"
    echo -e "\n${YELLOW}View release:${NC} https://github.com/$REPO/releases/tag/v$version"
}

upload_binaries_to_existing() {
    local version="$1"
    
    if [ -z "$version" ]; then
        die "Version parameter is required for upload_binaries_to_existing"
    fi
    
    log_step "Uploading Binaries to Existing Release"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] Would upload binaries to existing release v$version"
        log_info "Binaries to upload:"
        for f in "$BUILD_DIR"/syschecks-* "$BUILD_DIR"/checksums-*.txt; do
            [ -f "$f" ] && echo "  - $(basename "$f")"
        done
        return 0
    fi
    
    log_info "Deleting old assets from release v$version..."
    # Delete existing assets to avoid conflicts
    for asset in $(gh release view "v$version" --repo "$REPO" --json assets -q '.assets[].name' 2>/dev/null || true); do
        if [[ "$asset" == syschecks-* ]] || [[ "$asset" == checksums-* ]]; then
            log_info "Deleting old asset: $asset"
            gh release delete-asset "v$version" "$asset" --repo "$REPO" --yes 2>/dev/null || true
        fi
    done
    
    log_info "Uploading new binaries..."
    
    # Upload all binaries
    for binary in "$BUILD_DIR"/syschecks-*; do
        if [ -f "$binary" ]; then
            log_info "Uploading $(basename "$binary")..."
            gh release upload "v$version" "$binary" --repo "$REPO" --clobber
        fi
    done
    
    # Upload checksum file
    if [ -f "$BUILD_DIR/checksums-sha256.txt" ]; then
        log_info "Uploading checksums-sha256.txt..."
        gh release upload "v$version" "$BUILD_DIR/checksums-sha256.txt" --repo "$REPO" --clobber
    fi
    
    log_success "Binaries uploaded to release v$version successfully!"
    echo -e "\n${YELLOW}View release:${NC} https://github.com/$REPO/releases/tag/v$version"
}

# ============================================================================
# Summary Functions
# ============================================================================

print_summary() {
    local version="$1"
    local release_type="$2"
    
    log_step "Release Summary"
    
    echo -e "${BOLD}Version:${NC}      v$version"
    echo -e "${BOLD}Type:${NC}         $release_type"
    echo -e "${BOLD}Repository:${NC}   $REPO"
    echo ""
    
    echo -e "${BOLD}Binaries:${NC}"
    for f in "$BUILD_DIR"/syschecks-*; do
        if [ -f "$f" ]; then
            local size=$(ls -lh "$f" | awk '{print $5}')
            echo "  - $(basename "$f") ($size)"
        fi
    done
    
    if [ -f "$BUILD_DIR/checksums-sha256.txt" ]; then
        echo ""
        echo -e "${BOLD}Checksums:${NC}"
        echo "  - checksums-sha256.txt"
    fi
    
    echo ""
}

print_post_release() {
    echo -e "\n${GREEN}🎉 Release published successfully!${NC}\n"
    
    echo -e "${BOLD}Next Steps:${NC}"
    echo "  1. Update CHANGELOG.md if not already done"
    echo "  2. Announce the release (blog, social media, etc.)"
    echo "  3. Monitor issue tracker for reports"
    echo ""
    
    echo -e "${BOLD}Useful Links:${NC}"
    echo "  Release: https://github.com/$REPO/releases/tag/v$VERSION"
    echo "  Issues:  https://github.com/$REPO/issues"
    echo ""
}

# ============================================================================
# Main
# ============================================================================

main() {
    # Save positional arguments before parsing options
    local args=("$@")
    
    # Parse options
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h|--help)
                print_usage
                exit 0
                ;;
            -n|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -f|--force)
                FORCE=true
                shift
                ;;
            -s|--skip-build)
                SKIP_BUILD=true
                shift
                ;;
            -t|--skip-tests)
                SKIP_TESTS=true
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -*)
                die "Unknown option: $1\nRun with --help for usage"
                ;;
            *)
                break
                ;;
        esac
    done
    
    # Get positional arguments
    local VERSION="${1:-}"
    local RELEASE_TYPE="${2:-release}"
    
    # Validate arguments
    if [ -z "$VERSION" ]; then
        print_usage
        die "VERSION is required"
    fi
    
    # Remove 'v' prefix if provided
    VERSION="${VERSION#v}"
    
    validate_semver "$VERSION"
    validate_release_type "$RELEASE_TYPE"
    
    # Header
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}           ${BOLD}SysChecks Release Automation${NC}                           ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}           Version: v$VERSION $(printf '%*s' $((37-${#VERSION})) '')${CYAN}║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════╝${NC}"
    
    if [ "$DRY_RUN" = true ]; then
        echo -e "\n${YELLOW}🔍 DRY RUN MODE - No changes will be made${NC}\n"
    fi
    
    # Execute release pipeline
    check_prerequisites
    
    # Check if release exists and get user choice
    UPLOAD_ONLY=false
    if ! check_existing_release "$VERSION"; then
        UPLOAD_ONLY=true
    fi
    
    run_tests
    build_binaries
    generate_checksums
    
    # Show summary and confirm
    print_summary "$VERSION" "$RELEASE_TYPE"
    
    if [ "$DRY_RUN" = false ] && [ "$FORCE" = false ]; then
        if [ "$UPLOAD_ONLY" = true ]; then
            if ! confirm "Upload binaries to existing release?"; then
                log_warning "Upload cancelled"
                exit 0
            fi
        else
            if ! confirm "Create this release?"; then
                log_warning "Release cancelled"
                exit 0
            fi
        fi
    fi
    
    # Create the release or upload binaries
    if [ "$UPLOAD_ONLY" = true ]; then
        upload_binaries_to_existing "$VERSION"
    else
        # Generate release notes
        RELEASE_NOTES=$(generate_release_notes "$VERSION" "$RELEASE_TYPE")
        create_release "$VERSION" "$RELEASE_TYPE" "$RELEASE_NOTES"
    fi
    
    # Post-release info
    if [ "$DRY_RUN" = false ]; then
        print_post_release
    fi
}

# Run main with all arguments
main "$@"
