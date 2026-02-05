#!/bin/bash
# recover.sh - Update installer and rescue/rollback script
#
# This script is called by systemd as ExecStartPre before the simtezilo service starts.
# It has two main responsibilities:
# 1. Apply pending updates by calling the platform command
# 2. Track consecutive failures and perform automatic rollback if failures exceed threshold
#
# Design principles:
# - Minimal external dependencies (no jq, python, etc.)
# - Simple text file for failure counter
# - Delegates update logic to the platform command for proper JSON handling
# - Fail-safe: errors are logged but don't prevent operation
#
# Note: The platform command handles installation failures with atomic staging.
# This script handles runtime failures (e.g., app panics after a successful update)
# by rolling back to the previous version after repeated startup failures.

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

BASE_DIR="${BASE_DIR:-/opt/simtezilo}"
BIN_DIR="${BASE_DIR}/bin"
DATA_DIR="${BASE_DIR}/data"
UPDATE_DIR="${DATA_DIR}/update"
ROLLBACK_ARCHIVE="${UPDATE_DIR}/rollback.tgz"
COUNTER_FILE="${DATA_DIR}/failed_start.counter"
PLATFORM_CMD="${BIN_DIR}/platform"
LOG_TAG="simtezilo-recover"

# Maximum consecutive startup failures before automatic rollback
MAX_FAILURES=10

# =============================================================================
# Functions
# =============================================================================

log() {
    logger -t "$LOG_TAG" "$@" 2>/dev/null || true
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $@" >&2
}

# Read current startup failure count from counter file
read_counter() {
    if [[ -f "$COUNTER_FILE" ]]; then
        local count
        count=$(cat "$COUNTER_FILE" 2>/dev/null || echo "0")
        # Validate it's a number
        if [[ "$count" =~ ^[0-9]+$ ]]; then
            echo "$count"
        else
            echo "0"
        fi
    else
        echo "0"
    fi
}

# Increment startup failure counter
increment_counter() {
    local count
    count=$(read_counter)
    count=$((count + 1))
    
    mkdir -p "$DATA_DIR" 2>/dev/null || true
    echo "$count" > "$COUNTER_FILE" 2>/dev/null || true
    
    echo "$count"
}

# Reset startup failure counter to zero
reset_counter() {
    rm -f "$COUNTER_FILE" 2>/dev/null || true
}

# Check if there's a pending update and apply it using the platform command
apply_pending_update() {
    if [[ ! -x "$PLATFORM_CMD" ]]; then
        log "Platform command not found at $PLATFORM_CMD - skipping update check"

        return 0
    fi

    log "Checking for pending updates via platform command"
    
    # Run the platform update-apply command
    # It handles all the JSON parsing, update logic, and failure recovery properly
    local output
    if output=$("$PLATFORM_CMD" -b "$BASE_DIR" update-apply 2>&1); then
        log "Update apply completed successfully"

        # Reload systemd in case the service file was updated
        systemctl daemon-reload 2>/dev/null || true

        # Reset failure counter on successful update
        reset_counter

        return 0
    else
        local exit_code=$?
        log "Update apply returned exit code $exit_code: $output"

        return $exit_code
    fi
}

# Perform emergency rollback from archive
perform_rollback() {
    if [[ ! -f "$ROLLBACK_ARCHIVE" ]]; then
        log "ERROR: No rollback archive available at $ROLLBACK_ARCHIVE"

        return 1
    fi

    log "Performing emergency rollback from archive"

    # Extract archive directly to base directory, overwriting existing files
    if ! tar -xzf "$ROLLBACK_ARCHIVE" -C "$BASE_DIR" 2>/dev/null; then
        log "ERROR: Failed to extract rollback archive"

        return 1
    fi

    # Reload systemd in case the service file was restored
    systemctl daemon-reload 2>/dev/null || true

    # Reset counter after successful rollback
    reset_counter
    
    log "Rollback complete - restored previous version from archive"

    return 0
}

# =============================================================================
# Main
# =============================================================================

main() {
    # First, check for and apply any pending updates
    # This happens before failure counting so a pending update gets a chance to run
    # The platform command handles all states including:
    # - pending: Apply the update
    # - installing: Restore from staging (interrupted install)
    # - failed: Report status (user must manually retry installation)
    # - complete: Clean up
    apply_pending_update || true

    # Check if rollback archive exists for failure recovery
    if [[ ! -f "$ROLLBACK_ARCHIVE" ]]; then
        log "No rollback archive present - skipping failure recovery check"

        exit 0
    fi

    # Read and increment failure counter
    local fail_count
    fail_count=$(increment_counter)
    
    log "Failure count: $fail_count (threshold: $MAX_FAILURES)"

    # Check if we've exceeded the failure threshold
    # This catches runtime failures (e.g., app panics) after a successful update
    if [[ "$fail_count" -ge "$MAX_FAILURES" ]]; then
        log "ALERT: Failure threshold exceeded - triggering automatic rollback"
        
        if perform_rollback; then
            log "Emergency rollback successful - system should recover on next start"

            exit 0
        else
            log "CRITICAL: Emergency rollback failed - manual intervention required"

            exit 1
        fi
    fi
    
    exit 0
}

main "$@"
