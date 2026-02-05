#!/bin/bash
# Version management script for Simtezilo
# https://www.conventionalcommits.org/

set -e

VERSION_FILE="VERSION"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

error() {
    echo -e "${RED}ERROR: $1${NC}" >&2
    exit 1
}

success() {
    echo -e "${GREEN}✓ $1${NC}"
}

info() {
    echo -e "$1"
}

# Validate VERSION file exists and has correct format
validate_version() {
    if [ ! -f "$VERSION_FILE" ]; then
        error "VERSION file not found"
    fi
    
    version=$(cat "$VERSION_FILE")
    # Support semantic versioning with optional pre-release and build metadata
    # Examples: v1.0.0, v1.0.0-alpha, v1.0.0-beta.3, v1.0.0-rc.1+build.123
    if ! echo "$version" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
        error "VERSION must match format: v0.0.0[-suffix][+build] (found: $version)"
    fi
    
    success "VERSION file is valid: $version"
}

# Show current version
show_version() {
    if [ ! -f "$VERSION_FILE" ]; then
        error "VERSION file not found"
    fi
    cat "$VERSION_FILE"
}

# Bump version by type
bump_version() {
    local bump_type=$1
    
    validate_version > /dev/null
    
    # Get current version and strip the 'v' prefix
    current=$(cat "$VERSION_FILE" | sed 's/^v//')
    
    # Split version into core (1.2.3) and suffix (-beta.1+build)
    core=$(echo "$current" | sed -E 's/(-.*|[+].*)//')
    suffix=$(echo "$current" | sed -E 's/^[0-9]+\.[0-9]+\.[0-9]+//')
    
    case $bump_type in
        patch)
            new=$(echo "$core" | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
            ;;
        minor)
            new=$(echo "$core" | awk -F. '{$2 = $2 + 1; $3 = 0} 1' | sed 's/ /./g')
            ;;
        major)
            new=$(echo "$core" | awk -F. '{$1 = $1 + 1; $2 = 0; $3 = 0} 1' | sed 's/ /./g')
            ;;
        *)
            error "Invalid bump type: $bump_type (must be patch, minor, or major)"
            ;;
    esac
    
    # Extract build metadata if present (starts with +)
    build_meta=""
    if echo "$suffix" | grep -q '^+'; then
        build_meta=$(echo "$suffix" | grep -oE '\+.*')
    elif echo "$suffix" | grep -q '\+'; then
        build_meta=$(echo "$suffix" | grep -oE '\+.*')
    fi
    
    # For minor/major: add -beta.1 automatically (pre-release first)
    # For patch: go directly to stable (remove pre-release)
    if [ "$bump_type" = "patch" ]; then
        # Patch bumps go directly to stable, preserve build metadata
        echo "v${new}${build_meta}" > "$VERSION_FILE"
        if [ -n "$suffix" ]; then
            info "Version bumped: v$current -> v${new}${build_meta} (patch release, pre-release removed)"
        else
            info "Version bumped: v$current -> v${new}${build_meta}"
        fi
    else
        # Minor/Major bumps automatically become beta.1 pre-releases
        echo "v${new}-beta.1${build_meta}" > "$VERSION_FILE"
        info "Version bumped: v$current -> v${new}-beta.1${build_meta} (pre-release for testing)"
    fi
}

# Remove pre-release suffix to create stable release
release_version() {
    validate_version > /dev/null
    
    # Get current version and strip the 'v' prefix
    current=$(cat "$VERSION_FILE" | sed 's/^v//')
    
    # Split version into components
    core=$(echo "$current" | sed -E 's/(-.*|[+].*)//')
    full_suffix=$(echo "$current" | sed -E 's/^[0-9]+\.[0-9]+\.[0-9]+//')
    
    # Extract pre-release part (before any +)
    prerelease=$(echo "$full_suffix" | sed -E 's/\+.*//')
    # Extract build metadata (after +)
    build=$(echo "$full_suffix" | grep -oE '\+.*' || echo "")
    
    if [ -z "$prerelease" ]; then
        info "Already a stable release: v$current"
        return 0
    fi
    
    # Remove pre-release, keep build metadata
    new_version="v${core}${build}"
    echo "$new_version" > "$VERSION_FILE"
    info "Released as stable: v$current -> $new_version"
}

# Bump or add pre-release version
prerelease_version() {
    local label=${1:-beta}
    
    validate_version > /dev/null
    
    # Get current version and strip the 'v' prefix
    current=$(cat "$VERSION_FILE" | sed 's/^v//')
    
    # Split version into components
    core=$(echo "$current" | sed -E 's/(-.*|[+].*)//')
    full_suffix=$(echo "$current" | sed -E 's/^[0-9]+\.[0-9]+\.[0-9]+//')
    
    # Extract pre-release part (before any +)
    prerelease=$(echo "$full_suffix" | sed -E 's/\+.*//')
    # Extract build metadata (after +)
    build=$(echo "$full_suffix" | grep -oE '\+.*' || echo "")
    
    if [ -z "$prerelease" ]; then
        # No pre-release, add one
        new_version="v${core}-${label}.1${build}"
        echo "$new_version" > "$VERSION_FILE"
        info "Pre-release added: v$current -> $new_version"
    else
        # Has pre-release, check if same label
        current_label=$(echo "$prerelease" | sed -E 's/^-([^.]+).*/\1/')
        current_number=$(echo "$prerelease" | sed -E 's/^-[^.]+\.([0-9]+).*/\1/')
        
        if [ "$current_label" = "$label" ]; then
            # Same label, increment number
            if [ -n "$current_number" ] && [ "$current_number" != "$prerelease" ]; then
                new_number=$((current_number + 1))
                new_version="v${core}-${label}.${new_number}${build}"
                echo "$new_version" > "$VERSION_FILE"
                info "Pre-release bumped: v$current -> $new_version"
            else
                # Has label but no number, add .1
                new_version="v${core}-${label}.1${build}"
                echo "$new_version" > "$VERSION_FILE"
                info "Pre-release number added: v$current -> $new_version"
            fi
        else
            # Different label, switch to new label with .1
            new_version="v${core}-${label}.1${build}"
            echo "$new_version" > "$VERSION_FILE"
            info "Pre-release label changed: v$current -> $new_version"
        fi
    fi
}

# Analyze git commits to determine appropriate version bump
analyze_commits() {
    local quiet=$1
    
    # Get the last tag or use the VERSION file
    last_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    
    # If no git tag exists, use VERSION file as baseline
    if [ -z "$last_tag" ]; then
        commit_range="HEAD"
        [ -z "$quiet" ] && info "No git tags found, analyzing all commits since beginning"
    else
        commit_range="${last_tag}..HEAD"
        [ -z "$quiet" ] && info "Analyzing commits since ${last_tag}"
    fi
    
    # Get commits since last tag/version
    commits=$(git log "$commit_range" --pretty=format:"%s" 2>/dev/null || echo "")
    
    if [ -z "$commits" ]; then
        [ -z "$quiet" ] && info "No new commits found"
        return 0
    fi
    
    # Initialize bump type (0=none, 1=patch, 2=minor, 3=major)
    bump_type=0
    
    # Analyze each commit
    while IFS= read -r commit; do
        [ -z "$quiet" ] && info "  Checking: $commit"
        
        # Check for BREAKING CHANGE (major bump)
        if echo "$commit" | grep -qiE "^[a-z]+(\([a-z0-9_-]+\))?!:|BREAKING CHANGE:"; then
            [ -z "$quiet" ] && info "    → Found BREAKING CHANGE (major)"
            bump_type=3
            continue
        fi
        
        # Check for feat: (minor bump)
        if echo "$commit" | grep -qE "^feat(\([a-z0-9_-]+\))?:"; then
            [ -z "$quiet" ] && info "    → Found feature (minor)"
            [ $bump_type -lt 2 ] && bump_type=2
            continue
        fi
        
        # Check for fix: (patch bump)
        if echo "$commit" | grep -qE "^fix(\([a-z0-9_-]+\))?:"; then
            [ -z "$quiet" ] && info "    → Found fix (patch)"
            [ $bump_type -lt 1 ] && bump_type=1
            continue
        fi
        
        # Check for perf: (patch bump - performance improvements)
        if echo "$commit" | grep -qE "^perf(\([a-z0-9_-]+\))?:"; then
            [ -z "$quiet" ] && info "    → Found performance improvement (patch)"
            [ $bump_type -lt 1 ] && bump_type=1
            continue
        fi
    done <<< "$commits"
    
    # Determine the bump type string
    case $bump_type in
        0)
            [ -z "$quiet" ] && info ""
            [ -z "$quiet" ] && info "No version bump needed (no feat/fix/BREAKING commits found)"
            return 0
            ;;
        1)
            [ -z "$quiet" ] && info ""
            [ -z "$quiet" ] && info "Recommended: PATCH version bump"
            echo "patch"
            ;;
        2)
            [ -z "$quiet" ] && info ""
            [ -z "$quiet" ] && info "Recommended: MINOR version bump"
            echo "minor"
            ;;
        3)
            [ -z "$quiet" ] && info ""
            [ -z "$quiet" ] && info "Recommended: MAJOR version bump"
            echo "major"
            ;;
    esac
}

# Automatically determine and apply version bump
auto_bump() {
    validate_version > /dev/null
    
    # Capture only the last line (the bump type) from analyze_commits
    output=$(analyze_commits)
    bump_type=$(echo "$output" | tail -n1)
    
    if [ -n "$bump_type" ] && [ "$bump_type" != "0" ]; then
        bump_version "$bump_type"
    else
        info "No version bump needed"
    fi
}

# Check what version would be bumped to (dry-run)
check_version() {
    validate_version
    analyze_commits > /dev/null
}

# Create git tag matching VERSION file
tag_version() {
    validate_version > /dev/null
    
    version=$(cat "$VERSION_FILE")
    
    if git rev-parse "$version" >/dev/null 2>&1; then
        error "Tag $version already exists"
    fi
    
    git tag -a "$version" -m "Release $version"
    success "Created tag $version"
}

# Show usage
show_usage() {
    cat << EOF
Usage: $(basename "$0") [COMMAND] [OPTIONS]

Version management for Simtezilo using conventional commits and semantic versioning.

Commands: 
    check               Analyze commits and show recommended bump (dry-run)
    patch               Manually bump patch version (0.8.0 -> 0.8.1, stable release)
    minor               Manually bump minor version (0.8.0 -> 0.9.0-beta.1, pre-release)
    major               Manually bump major version (0.8.0 -> 1.0.0-beta.1, pre-release)
    prerelease [label]  Add or bump pre-release version (default label: beta)
    release             Remove pre-release suffix for stable release (0.9.0-beta.2 -> 0.9.0)
    validate            Validate VERSION file format
    show                Display current version
    tag                 Create git tag matching VERSION file
    help                Show this help message

Version Format (Semantic Versioning):
    v<major>.<minor>.<patch>[-<pre-release>][+<build>]

    Build metatdata is not managed by this script but can be manually added in the version file.

Version Update Behavior:
    - patch: Direct to stable, removes pre-release suffix (bug fixes)
    - minor/major: Automatically adds -beta.1 (requires testing)
    - prerelease: Adds or increments pre-release version
    - release: Removes pre-release suffix, keeps version number
    - Build metadata (+build) is always preserved when present
    
Examples:
    $(basename "$0") auto             # Auto-bump based on commits
    $(basename "$0") check            # Show what would be bumped
    $(basename "$0") minor            # v0.8.0 -> v0.9.0-beta.1
    $(basename "$0") prerelease       # v0.9.0-beta.1 -> v0.9.0-beta.2
    $(basename "$0") release          # v0.9.0-beta.2 -> v0.9.0
    $(basename "$0") patch            # v0.9.0 -> v0.9.1 (stable)
    $(basename "$0") prerelease rc    # Add/bump release candidate
    $(basename "$0") tag              # Create git tag

Version Bump Rules (Conventional Commits):
    fix:, perf:              → PATCH version bump
    feat:                    → MINOR version bump
    BREAKING CHANGE:, feat!: → MAJOR version bump

EOF
}

# Main command dispatcher
case "${1:-help}" in
    auto)
        auto_bump
        ;;
    check)
        check_version
        ;;
    patch|minor|major)
        bump_version "$1"
        ;;
    prerelease|pre-release)
        prerelease_version "$2"
        ;;
    release)
        release_version
        ;;
    validate)
        validate_version
        ;;
    show)
        show_version
        ;;
    tag)
        tag_version
        ;;
    help|--help|-h)
        show_usage
        ;;
    *)
        error "Unknown command: $1\n\nRun '$(basename "$0") help' for usage information."
        ;;
esac
