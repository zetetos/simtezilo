#!/bin/bash
# version.sh - Shared version utilities for build scripts
#
# Source this file in other scripts:
#   source "$(dirname "$0")/lib/version.sh"
#
# Provides:
#   VERSION_FILE    - Path to VERSION file
#   VERSION_RAW     - Raw version string from file (e.g., "v0.8.0-beta.1")
#   VERSION         - Version without 'v' prefix (e.g., "0.8.0-beta.1")
#   VERSION_TAG     - Version with 'v' prefix (e.g., "v0.8.0-beta.1")
#   VERSION_CORE    - Core version without pre-release (e.g., "0.8.0")
#   VERSION_PRERELEASE - Pre-release suffix (e.g., "-beta.1") or empty
#   VERSION_CHANNEL - Derived channel: stable, beta, or dev
#
# Functions:
#   version_error   - Print error message and exit
#   version_info    - Print info message
#   version_success - Print success message
#   version_load    - Load and parse version from VERSION file
#   version_channel - Derive channel from a version string

# Prevent multiple sourcing
if [[ -n "${_VERSION_LIB_LOADED:-}" ]]; then
    return 0
fi
_VERSION_LIB_LOADED=1

# Version file location (can be overridden before sourcing)
VERSION_FILE="${VERSION_FILE:-VERSION}"

# Colors for output
_RED='\033[0;31m'
_GREEN='\033[0;32m'
_YELLOW='\033[1;33m'
_NC='\033[0m'

version_error() {
    echo -e "${_RED}ERROR: $1${_NC}" >&2
    exit 1
}

version_info() {
    echo -e "$1"
}

version_success() {
    echo -e "${_GREEN}✓ $1${_NC}"
}

# Derive channel from version string
# Args: version string (with or without 'v' prefix)
# Returns: channel name via echo
version_channel() {
    local ver="${1#v}"
    
    if [[ "$ver" =~ -([a-zA-Z]+) ]]; then
        local label="${BASH_REMATCH[1]}"
        case "$label" in
            alpha|dev)
                echo "dev"
                ;;
            beta|rc)
                echo "beta"
                ;;
            *)
                echo "dev"  # Unknown pre-release goes to dev
                ;;
        esac
    else
        echo "stable"
    fi
}

# Load and parse version from VERSION file
# Sets: VERSION_RAW, VERSION, VERSION_TAG, VERSION_CORE, VERSION_PRERELEASE, VERSION_CHANNEL
version_load() {
    if [[ ! -f "$VERSION_FILE" ]]; then
        version_error "VERSION file not found: $VERSION_FILE"
    fi
    
    VERSION_RAW=$(cat "$VERSION_FILE" 2>/dev/null || echo "")
    if [[ -z "$VERSION_RAW" ]]; then
        version_error "VERSION file is empty"
    fi
    
    # Validate format
    if ! echo "$VERSION_RAW" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
        version_error "VERSION must match format: v0.0.0[-suffix][+build] (found: $VERSION_RAW)"
    fi
    
    # Parse components
    VERSION="${VERSION_RAW#v}"
    VERSION_TAG="v${VERSION}"
    
    # Extract core version (before any - or +)
    VERSION_CORE=$(echo "$VERSION" | sed -E 's/(-.*|\+.*)//')
    
    # Extract pre-release (between - and +, or after - if no +)
    if [[ "$VERSION" =~ -([^+]+) ]]; then
        VERSION_PRERELEASE="-${BASH_REMATCH[1]}"
    else
        VERSION_PRERELEASE=""
    fi
    
    # Derive channel
    VERSION_CHANNEL=$(version_channel "$VERSION")
    
    # Export for subshells
    export VERSION_RAW VERSION VERSION_TAG VERSION_CORE VERSION_PRERELEASE VERSION_CHANNEL
}

# Auto-load version if VERSION file exists in current directory
if [[ -f "$VERSION_FILE" ]]; then
    version_load
fi
