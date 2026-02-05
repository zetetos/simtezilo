# Simtezilo Update System

This document describes how the Simtezilo update system works, including the update checking, downloading, staging, and installation processes.

## Architecture Overview

The update system consists of two main components:

1. **`app/updater` package** (Go):      Runs within the simtezilo application to check for updates, download them, and stage them for installation
2. **`platform update-apply`** (Go):    Runs at service startup via systemd to extract archives, perform atomic installation with staging, and handle failure recovery

This two-phase approach ensures safe updates:
- The running application never replaces itself while running
- The platform command handles all system-specific installation logic with atomic operations
- Failed installations are automatically recovered via staging directory rollback

## Update Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              UPDATE LIFECYCLE                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌───────────────────┐    ┌───────────────────┐    ┌───────────────────┐   │
│   │       CHECK       │───▶│      DOWNLOAD     │───▶│       STAGE       │   │
│   │     (Checker)     │    │    (Downloader)   │    │    (Installer)    │   │
│   │   [simtezilo]     │    │   [simtezilo]     │    │   [simtezilo]     │   │
│   └───────────────────┘    └───────────────────┘    └───────────────────┘   │
│             │                                                 │             │
│             │                                                 ▼             │
│             │                                       ┌───────────────────┐   │
│             │                                       │ update-state.json │   │
│             │                                       │  status: pending  │   │
│             │                                       └───────────────────┘   │
│             │                                                 │             │
│  ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─ ─  SERVICE RESTART  ─ ─ ─ ─ ─ ─ ─ ─ ─│─ ─ ─ ─ ─ ─  │
│             │                                                 │             │
│             │                                                 ▼             │
│             │                                       ┌───────────────────┐   │
│             │                                       │    recover.sh     │   │
│             │                                       │  [update-apply]   │   │
│             │                                       └───────────────────┘   │
│             │                                                 │             │
│             ▼                                                 ▼             │
│   ┌───────────────────┐                             ┌───────────────────┐   │
│   │  CONFIRM SUCCESS  │◀────────────────────────────│  SERVICE STARTS   │   │
│   └───────────────────┘                             └───────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

### Simtezilo Application (app/updater package)

The main application handles **platform-agnostic** update operations:

| Component  | File            | Purpose                              |
|------------|-----------------|--------------------------------------|
| Updater    | `updater.go`    | Orchestrates the update workflow     |
| Checker    | `checker.go`    | Fetches manifests, compares versions |
| Downloader | `downloader.go` | Downloads and verifies archives      |
| Installer  | `installer.go`  | Manages state, stages updates        |

**Responsibilities:**
- Check for available updates
- Download update archives
- Verify checksums
- Write state file for platform command
- Request service restart
- Confirm successful update after restart

### Platform Command (`platform update-apply`)

The platform command handles **system-specific** installation with atomic operations:

**Responsibilities:**
- Read state file
- Verify checksums
- Extract archives (tar.gz, zip)
- Perform atomic installation using staging directory
- Handle interrupted installations (restore from staging)
- Create rollback archive from staged original files
- Update state file
- Handle complete/failed states

### Helper Script (`recover.sh`)

A bash script that invokes the platform command and provides emergency rollback:

**Responsibilities:**
- Invoke `platform update-apply` command
- Track consecutive startup failures
- Perform automatic rollback after repeated failures (catches buggy updates)
- Reload systemd daemon after updates

## Update States

The system tracks updates through these states in `update-state.json`:

| Status       | Description                                         |
|--------------|-----------------------------------------------------|
| `pending`    | Update downloaded and staged, waiting for restart   |
| `installing` | Installation in progress (atomic staging active)    |
| `complete`   | Installation succeeded, awaiting confirmation       |
| `failed`     | Installation failed, user must manually retry       |
| `rolled_back`| Rolled back to previous version                     |

### State Transitions

```
pending ────► installing ────► complete ────► (cleared)
                 │                               
                 │ (failure)                     
                 ▼                               
              failed
                 │
                 │ (user retry via web UI)
                 ▼
              pending
```

### State File Format

```json
{
  "pendingVersion": "2.0.0",
  "currentVersion": "1.5.0",
  "downloadPath": "/opt/simtezilo/data/update/downloads/simtezilo-2.0.0-linux-arm64.tar.gz",
  "extractDir": "/opt/simtezilo/data/update/extract",
  "sha256": "abc123...",
  "timestamp": "2026-01-16T10:30:00Z",
  "status": "pending",
  "failCount": 0,
  "lastError": ""
}
```

## Phase 1: Application-Side (simtezilo)

When the application detects and downloads an update:

1. **Check**    - Fetches manifest from update server, compares versions
2. **Download** - Downloads the archive, verifies SHA256 checksum
3. **Prepare**  - Saves state file with `status: "pending"`
4. **Wait**     - Application continues running; update applied on next restart

```go
// Simplified flow in the updater package
info, _ := updater.CheckNow()                          // Check for updates
updater.Download(ctx, info, callback)                  // Download archive  
installer.Prepare(downloadPath, info, currentVersion)  // Stage for install
```

## Phase 2: Installation (platform update-apply)

The `platform update-apply` command runs as a systemd `ExecStartPre` hook.

### Archive Layout

The update archive must follow this structure:

```
simtezilo/
├── bin/
│   ├── simtezilo          # Main binary (INSTALLED)
│   └── platform           # Helper binary (INSTALLED)
├── init/
│   ├── recover.sh         # Helper script (INSTALLED)
│   └── simtezilo.service  # Systemd unit (INSTALLED)
├── etc/
│   └── simtezilo.conf     # Config file (NOT overwritten if exists)
└── data/
    └── replays/           # User data (NOT overwritten)
```

### Atomic Installation Process

The installation uses a staging directory for atomic operations:

1. **Load state**              - Read update-state.json
2. **Verify download**         - Check file exists and checksum matches
3. **Mark installing**         - Set status to "installing"
4. **Extract archive**         - Extract to temporary directory
5. **Create staging dir**      - Create `data/update/staging/`
6. **Install files atomically**:
   - For each file to install:
     - Move original → staging (atomic)
     - Move new → destination (atomic)
   - Install order: init scripts, config, additional binaries, **main binary last**
7. **On success**:
   - Create rollback archive from staging
   - Clean up staging and extract directories
   - Mark status "complete"
8. **On failure**:
   - Restore all files from staging
   - Clean up
   - Mark status "failed"

### Handling Interrupted Installation

If the system reboots or crashes during installation (status = "installing"):

1. Platform command detects "installing" status on next boot
2. Walks staging directory and restores all files to original locations
3. Cleans up staging and extract directories
4. Marks status "failed" with reason "interrupted installation"

### Rollback Archive

After successful installation, the original files are preserved in `rollback.tgz`:

- Created from the staging directory contents
- Contains original versions of all replaced files
- Used by `recover.sh` for automatic rollback on runtime failures
- Can be used for manual rollback via `platform update-rollback`
- Automatically deleted after `installer.ConfirmSuccess()` is called

### Success Confirmation

After successful startup, the application calls `installer.ConfirmSuccess()`:

1. Removes `rollback.tgz` archive (no longer needed)
2. Clears the state file
3. Resets startup failure counter
4. Update cycle complete

## Failure Handling

There are two types of failures handled by different components:

### Installation Failures (platform command)

When an update fails to install (extraction error, file copy error, etc.):

- Platform command restores files from staging directory
- Status is set to "failed" with error reason in `lastError`
- User must manually trigger retry via web UI
- No automatic retry prevents boot loops from repeatedly failing installs

### Runtime Failures (recover.sh)

When a successfully installed update causes the application to crash repeatedly:

- `recover.sh` tracks consecutive startup failures via counter file
- After 10 consecutive failures, automatic rollback is triggered
- Previous version is restored from `rollback.tgz`
- This catches bugs that cause panics or immediate crashes

### User Actions for Failed Updates

Via the web UI, users can:
1. **Retry** - Resets status to "pending" and retries on next restart
2. **Delete** - Clears the update state, removes downloaded archive
3. **Rollback** - Manually restore previous version from rollback archive

## Configuration

### Environment Variables (recover.sh)

| Variable       | Default          | Description                              |
|----------------|------------------|------------------------------------------|
| `BASE_DIR`     | `/opt/simtezilo` | Base installation directory              |
| `MAX_FAILURES` | `10`             | Consecutive failures before rollback     |

### Platform Command Directories

| Directory    | Path                           | Description                      |
|--------------|--------------------------------|----------------------------------|
| Install      | `{base}/bin`                   | Binary installation directory    |
| Init         | `{base}/init`                  | Init scripts directory           |
| Etc          | `{base}/etc`                   | Configuration files directory    |
| Update data  | `{base}/data/update`           | State, downloads, staging        |
| Staging      | `{base}/data/update/staging`   | Atomic staging directory         |
| Extract      | `{base}/data/update/extract`   | Temporary archive extraction     |

### Application Config

```go
updater.Config{
    Enabled:         true,
    BaseURL:         "https://updates.example.com",
    Channel:         "stable",
    CheckInterval:   1 * time.Hour,
    AutoInstall:     false,
    InstallDir:      "/opt/simtezilo/bin",
    InitDir:         "/opt/simtezilo/init",
    DataDir:         "/opt/simtezilo/data/update",
    BinaryName:      "simtezilo",
    ServiceName:     "simtezilo",
    UseSystemd:      true,
}
```

## Systemd Integration

The service unit should include the recover.sh script as ExecStartPre:

```ini
[Service]
ExecStartPre=/opt/simtezilo/init/recover.sh
ExecStart=/opt/simtezilo/bin/simtezilo
Restart=on-failure
RestartSec=5
```

The recover.sh script:
1. Calls `platform update-apply` to handle any pending/failed/installing states
2. Tracks startup attempts for monitoring purposes
3. Reloads systemd daemon if service file was updated

---

# Custom Update Upload with Metadata

## Overview

When uploading custom update files via the web UI, you can include metadata to provide proper version information, changelog, and release date. This metadata will be displayed in the update panel just like manifest-based updates.

## Metadata File Format

Include a file named `manifest.json` in the root of your archive (zip or tar.gz) with the following structure:

```json
{
  "version": "1.2.3",
  "releaseDate": "2026-01-11T10:30:00Z",
  "changelog": [
    "## Version 1.2.3",
    "",
    "### New Features",
    "- Feature 1",
    "- Feature 2",
    "",
    "### Bug Fixes",
    "- Fix 1",
    "- Fix 2"
  ],
  "platform": "darwin-arm64"
}
```

### Fields

- **version** (required):     The version string (e.g., "1.2.3")
- **releaseDate** (required): ISO 8601 formatted date/time
- **changelog** (required):   Array of strings, each element is a line of the changelog (supports Markdown)
- **platform** (optional):    Target platform identifier

## Creating an Archive with Metadata

### For tar.gz archives:

```bash
# Create manifest.json
cat > manifest.json << 'EOF'
{
  "version": "1.2.3",
  "releaseDate": "2026-01-11T10:30:00Z",
  "changelog": [
    "## What's New",
    "",
    "- Improved performance",
    "- Fixed bugs"
  ],
  "platform": "darwin-arm64"
}
EOF

# Create archive with metadata and binary
tar czf simtezilo-1.2.3-darwin-arm64.tar.gz manifest.json simtezilo

# Clean up
rm manifest.json
```

### For zip archives:

```bash
# Create manifest.json (same as above)
cat > manifest.json << 'EOF'
{
  "version": "1.2.3",
  "releaseDate": "2026-01-11T10:30:00Z",
  "changelog": [
    "## What's New",
    "",
    "- Improved performance",
    "- Fixed bugs"
  ],
  "platform": "darwin-arm64"
}
EOF

# Create archive with metadata and binary
zip simtezilo-1.2.3-darwin-arm64.zip manifest.json simtezilo

# Clean up
rm manifest.json
```

## Upload Behavior

1. **With Metadata**: If `manifest.json` is found in the archive:
   - Version, changelog, and release date are extracted and displayed
   - The update panel shows the extracted information
   - File is saved with "custom-" prefix

2. **Without Metadata**: If no metadata file is found:
   - Falls back to synthetic version: "custom-{filename}"
   - Changelog shows: "Custom uploaded file: {filename}"
   - Release date is set to current time
   - File is saved with "custom-" prefix

## File Naming

Uploaded files are automatically prefixed with `custom-` to distinguish them from manifest-downloaded updates:
- Original: `simtezilo-1.2.3-darwin-arm64.tar.gz`
- Saved as: `custom-simtezilo-1.2.3-darwin-arm64.tar.gz`

## Cleanup

When uploading a new custom file, all previous downloads (including other custom uploads) are automatically cleaned up from the downloads directory.

## Example

See `doc/manifest.example.json` for a complete example of the metadata file.
