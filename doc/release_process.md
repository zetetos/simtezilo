## Release Process

### Overview

The release process builds distribution archives for all platforms and generates the update manifest. All release artifacts are placed in a single directory structure ready for upload to the update server.

### Quick Release

```bash
# Build everything and generate manifest
make release

# Upload to server (single copy operation)
scp -r dist/releases/stable/ server:/var/www/updates/releases/
```

### Release Channels

Channels are **automatically derived** from the version string:

| Version Pattern    | Channel  | Example            |
|--------------------|----------|--------------------|
| `v1.0.0`           | stable   | Production release |
| `v1.0.0-beta.1`    | beta     | Beta testing       |
| `v1.0.0-rc.1`      | beta     | Release candidate  |
| `v1.0.0-alpha.1`   | dev      | Early development  |
| `v1.0.0-dev.1`     | dev      | Development builds |

### Output Structure

After running `make release`, the output directory is ready for direct upload:

```
dist/releases/stable/
├── latest.json                                    # Channel manifest (for update checks)
└── v0.8.0/
    ├── manifest.json                              # Version-specific manifest
    ├── simtezilo-v0.8.0-darwin-arm64.tar.gz
    ├── simtezilo-v0.8.0-darwin-arm64.tar.gz.sha256
    ├── simtezilo-v0.8.0-linux-amd64.tar.gz
    ├── simtezilo-v0.8.0-linux-amd64.tar.gz.sha256
    ├── simtezilo-v0.8.0-linux-arm64.tar.gz
    ├── simtezilo-v0.8.0-linux-arm64.tar.gz.sha256
    ├── simtezilo-v0.8.0-windows-amd64.zip
    └── simtezilo-v0.8.0-windows-amd64.zip.sha256
```

### Manifest Format

The `latest.json` manifest contains URLs, checksums, and sizes for each platform:

```json
{
  "version": "0.8.0",
  "releaseDate": "2026-01-31T02:30:00Z",
  "channel": "stable",
  "platforms": {
    "darwin-arm64": {
      "url": "https://static.simtezilo.com/releases/stable/v0.8.0/simtezilo-v0.8.0-darwin-arm64.tar.gz",
      "sha256": "2db6048c3edcd2911b7856a4adffff011e5f0f7e03af966c7a5e4614f0abb20d",
      "size": 27983027
    },
    "linux-arm64": { ... },
    "linux-amd64": { ... },
    "windows-amd64": { ... }
  }
}
```

### Step-by-Step Release

#### 1. Prepare the version

```bash
# Check current version
make version/show

# For a new feature release, bump minor version
make version/minor    # Creates v0.9.0-beta.1

# Test and iterate on beta...
make version/prerelease beta  # v0.9.0-beta.2

# When ready for stable release
make version/release  # v0.9.0
```

#### 2. Build and package

```bash
# Build all platform binaries and create archives
make release
```

This runs:
- `make dist` - Builds binaries and creates distribution archives
- `make release/manifest` - Generates the update manifest

#### 3. Verify the release

```bash
# Check the output
ls -la dist/releases/stable/v0.8.0/

# Verify manifest content
cat dist/releases/stable/latest.json
```

#### 4. Publish

**Using rclone (Cloudflare R2):**
```bash
# Upload to Cloudflare R2 (requires R2_REMOTE env var)
R2_REMOTE=r2:mybucket make release/publish
```

**Using scp:**
```bash
# Single command to upload everything
scp -r dist/releases/stable/ server:/var/www/updates/releases/

# Or for beta releases
scp -r dist/releases/beta/ server:/var/www/updates/releases/
```

#### 5. Tag the release

```bash
make version/tag
git push --tags
```

### Build Scripts

The release process uses these scripts in `build/scripts/`:

| Script                  | Purpose                                      |
|-------------------------|----------------------------------------------|
| `lib/version.sh`        | Shared version utilities (sourced by others) |
| `gen_dist.sh`           | Creates distribution archives                |
| `gen_release_manifest.sh` | Generates update manifest JSON             |
| `update_version.sh`     | Version bumping and management               |

### Environment Variables

| Variable      | Default                                 | Description                     |
|---------------|-----------------------------------------|---------------------------------|
| `BASE_URL`    | `https://static.simtezilo.com/releases` | Base URL for download links     |
| `OUT_DIR`     | `./out`                                 | Directory with built binaries   |
| `DIST_DIR`    | `./dist/releases`                       | Output directory for archives   |
| `MIN_VERSION` | (none)                                  | Minimum version to upgrade from |
| `CHANGELOG`   | (none)                                  | Release notes text              |
| `R2_REMOTE`   | (none)                                  | rclone remote name for R2 bucket|

### Changelog Generation

The project uses **git-cliff** with conventional commits to generate changelogs. Configuration is in `cliff.toml`. See [Version Management](version_management.md) for commit format details and how commit types map to changelog sections.

### Makefile Targets

```bash
make dist              # Create distribution archives
make release/manifest  # Generate update manifest
make release           # Both: dist + manifest
make release/publish   # Upload to Cloudflare R2 (requires R2_REMOTE)
```
