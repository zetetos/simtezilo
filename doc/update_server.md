# Simtezilo Update Server

This document describes how to set up a Hugo-based static site to host update manifests for Simtezilo devices, with binaries stored in Cloudflare R2.

## Architecture Overview

The update system uses a split architecture:

- **Hugo Site** (Cloudflare Pages): Hosts JSON manifest files generated from `releases.yaml`
- **Cloudflare R2** (`https://static.simtezilo.com/`): Hosts binary release files

This approach provides:
- Version-controlled release metadata with Git history and PR reviews
- CDN-optimized binary delivery via R2
- Single CI/CD pipeline for atomic releases
- Easy rollback by reverting Git commits

## Directory Structure

```
updates-site/
├── config.toml
├── content/
│   └── releases/
│       └── _index.md
├── data/
│   └── releases.yaml          # Release metadata (version controlled)
├── layouts/
│   └── releases/
│       └── list.json          # Template for manifest JSON
└── themes/
```

Binaries are stored separately in R2:
```
static.simtezilo.com/
└── releases/
    ├── v1.0.0/
    │   └── simtezilo-linux-arm64
    ├── v1.1.0/
    │   └── simtezilo-linux-arm64
    └── v1.2.0/
        └── simtezilo-linux-arm64
```

## Hugo Configuration

### config.toml

```toml
baseURL = "https://simtezilo.com"
languageCode = "en-us"
title = "Simtezilo Updates"

# Static files base URL (R2 bucket)
[params]
  staticBaseURL = "https://static.simtezilo.com"

[outputs]
  home = ["HTML"]
  section = ["HTML", "JSON"]

[mediaTypes]
  [mediaTypes."application/json"]
    suffixes = ["json"]

[outputFormats]
  [outputFormats.JSON]
    mediaType = "application/json"
    baseName = "latest"
    isPlainText = true
```

## Release Data

### data/releases.yaml

```yaml
# Latest stable release
stable:
  version: "1.2.0"
  releaseDate: "2025-01-15T10:00:00Z"
  minUpgradeVersion: "1.0.0"
  changelog: |
    ## What's New in v1.2.0
    
    - Improved haptic feedback responsiveness
    - Added support for new GT7 vehicles
    - Fixed audio crackling on Raspberry Pi 5
    - WebUI performance improvements
  platforms:
    linux-arm64:
      url: "https://static.simtezilo.com/releases/v1.2.0/simtezilo-linux-arm64"
      sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
      size: 15728640

# Beta channel
beta:
  version: "1.3.0-beta.1"
  releaseDate: "2025-01-20T10:00:00Z"
  minUpgradeVersion: "1.0.0"
  changelog: |
    ## What's New in v1.3.0-beta.1
    
    - Experimental: New engine haptic profiles
    - Preview: Discord voice integration
  platforms:
    linux-arm64:
      url: "https://static.simtezilo.com/releases/v1.3.0-beta.1/simtezilo-linux-arm64"
      sha256: "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456"
      size: 16000000

# Development channel
dev:
  version: "1.3.0-dev.42"
  releaseDate: "2025-01-22T15:30:00Z"
  minUpgradeVersion: "1.0.0"
  changelog: |
    ## Development Build
    
    - Latest development snapshot
    - May contain unstable features
  platforms:
    linux-arm64:
      url: "https://static.simtezilo.com/releases/v1.3.0-dev.42/simtezilo-linux-arm64"
      sha256: "deadbeef12345678901234567890123456789012345678901234567890abcdef"
      size: 16500000
```

## Templates

### content/releases/_index.md

```markdown
---
title: "Releases"
outputs:
  - HTML
  - JSON
---
```

### layouts/releases/list.json

This template generates the `latest.json` manifest file:

```go-html-template
{{- $channel := "stable" -}}
{{- with .Site.Params.channel -}}
  {{- $channel = . -}}
{{- end -}}
{{- $release := index .Site.Data.releases $channel -}}
{
  "version": {{ $release.version | jsonify }},
  "releaseDate": {{ $release.releaseDate | jsonify }},
  "channel": {{ $channel | jsonify }},
  "minUpgradeVersion": {{ $release.minUpgradeVersion | jsonify }},
  "changelog": {{ $release.changelog | jsonify }},
  "platforms": {
    {{- $first := true -}}
    {{- range $platform, $info := $release.platforms -}}
      {{- if not $first -}},{{- end -}}
      {{- $first = false }}
    {{ $platform | jsonify }}: {
      "url": {{ $info.url | jsonify }},
      "sha256": {{ $info.sha256 | jsonify }},
      "size": {{ $info.size }}
    }
    {{- end }}
  }
}
```

## Multi-Channel Setup

Hugo generates separate manifest files for each channel. The template reads the channel from build parameters.

### layouts/releases/list.json

This template generates channel-specific manifest files:

```go-html-template
{{- $channel := .Site.Params.channel | default "stable" -}}
{{- $release := index .Site.Data.releases $channel -}}
{{- $staticBase := .Site.Params.staticBaseURL | default "https://static.simtezilo.com" -}}
{
  "version": {{ $release.version | jsonify }},
  "releaseDate": {{ $release.releaseDate | jsonify }},
  "channel": {{ $channel | jsonify }},
  "minUpgradeVersion": {{ $release.minUpgradeVersion | jsonify }},
  "changelog": {{ $release.changelog | jsonify }},
  "platforms": {
    {{- $first := true -}}
    {{- range $platform, $info := $release.platforms -}}
      {{- if not $first -}},{{- end -}}
      {{- $first = false }}
    {{ $platform | jsonify }}: {
      "url": {{ $info.url | jsonify }},
      "sha256": {{ $info.sha256 | jsonify }},
      "size": {{ $info.size }}
    }
    {{- end }}
  }
}
```

### Build Script

Create `scripts/build-manifests.sh` to generate all channel manifests:

```bash
#!/bin/bash
set -e

# Build stable channel (default)
hugo --minify -d public/releases/stable --params channel=stable

# Build beta channel
hugo --minify -d public/releases/beta --params channel=beta

# Build dev channel  
hugo --minify -d public/releases/dev --params channel=dev

echo "All channel manifests generated"
```

## CI/CD Integration

### GitHub Actions Workflow

This workflow handles the complete release process:
1. Downloads the binary from the GitHub release
2. Uploads it to Cloudflare R2
3. Updates `releases.yaml` with new version info
4. Commits and pushes the change (which triggers site rebuild)

```yaml
name: Publish Release

on:
  release:
    types: [published]
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to publish (e.g., v1.2.0)'
        required: true
      channel:
        description: 'Release channel'
        required: true
        default: 'stable'
        type: choice
        options:
          - stable
          - beta
          - dev

env:
  R2_BUCKET: simtezilo-static
  R2_ENDPOINT: https://${{ secrets.CLOUDFLARE_ACCOUNT_ID }}.r2.cloudflarestorage.com

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout updates site
        uses: actions/checkout@v4
        with:
          repository: zetetos/simtezilo-updates
          token: ${{ secrets.UPDATES_REPO_TOKEN }}
      
      - name: Set version and channel
        id: vars
        run: |
          if [ "${{ github.event_name }}" == "release" ]; then
            echo "version=${{ github.event.release.tag_name }}" >> $GITHUB_OUTPUT
            # Determine channel from tag (e.g., v1.2.0-beta.1 -> beta)
            if [[ "${{ github.event.release.tag_name }}" == *"-beta"* ]]; then
              echo "channel=beta" >> $GITHUB_OUTPUT
            elif [[ "${{ github.event.release.tag_name }}" == *"-dev"* ]]; then
              echo "channel=dev" >> $GITHUB_OUTPUT
            else
              echo "channel=stable" >> $GITHUB_OUTPUT
            fi
          else
            echo "version=${{ github.event.inputs.version }}" >> $GITHUB_OUTPUT
            echo "channel=${{ github.event.inputs.channel }}" >> $GITHUB_OUTPUT
          fi
      
      - name: Download release binary
        run: |
          VERSION="${{ steps.vars.outputs.version }}"
          curl -L -o simtezilo-linux-arm64 \
            "https://github.com/zetetos/simtezilo/releases/download/${VERSION}/simtezilo-linux-arm64"
          
          # Calculate SHA256 and size
          echo "SHA256=$(sha256sum simtezilo-linux-arm64 | cut -d' ' -f1)" >> $GITHUB_ENV
          echo "SIZE=$(stat -c%s simtezilo-linux-arm64)" >> $GITHUB_ENV
      
      - name: Upload binary to R2
        uses: ryand56/r2-upload-action@latest
        with:
          r2-account-id: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
          r2-access-key-id: ${{ secrets.R2_ACCESS_KEY_ID }}
          r2-secret-access-key: ${{ secrets.R2_SECRET_ACCESS_KEY }}
          r2-bucket: ${{ env.R2_BUCKET }}
          source-dir: ./
          destination-dir: releases/${{ steps.vars.outputs.version }}
          output-file-url: true
        env:
          SOURCE_FILE: simtezilo-linux-arm64
      
      - name: Update releases.yaml
        run: |
          VERSION="${{ steps.vars.outputs.version }}"
          CHANNEL="${{ steps.vars.outputs.channel }}"
          
          # Install yq
          sudo wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
          sudo chmod +x /usr/local/bin/yq
          
          # Update the release metadata
          yq -i ".${CHANNEL}.version = \"${VERSION}\"" data/releases.yaml
          yq -i ".${CHANNEL}.releaseDate = \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"" data/releases.yaml
          yq -i ".${CHANNEL}.platforms.\"linux-arm64\".url = \"https://static.simtezilo.com/releases/${VERSION}/simtezilo-linux-arm64\"" data/releases.yaml
          yq -i ".${CHANNEL}.platforms.\"linux-arm64\".sha256 = \"${SHA256}\"" data/releases.yaml
          yq -i ".${CHANNEL}.platforms.\"linux-arm64\".size = ${SIZE}" data/releases.yaml
      
      - name: Commit and push
        run: |
          VERSION="${{ steps.vars.outputs.version }}"
          CHANNEL="${{ steps.vars.outputs.channel }}"
          
          git config user.name "GitHub Actions"
          git config user.email "actions@github.com"
          git add data/releases.yaml
          git commit -m "Release ${VERSION} to ${CHANNEL} channel"
          git push
      
      - name: Trigger site rebuild
        run: |
          # The push above will trigger Cloudflare Pages build
          # Or manually trigger via API:
          curl -X POST "https://api.cloudflare.com/client/v4/pages/webhooks/deploy_hooks/${{ secrets.CF_PAGES_DEPLOY_HOOK }}"
```

### Updates Site Build Workflow

In the updates site repository, add this workflow to build and deploy manifests:

```yaml
# .github/workflows/deploy.yml
name: Deploy Updates Site

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Hugo
        uses: peaceiris/actions-hugo@v2
        with:
          hugo-version: 'latest'
          extended: true
      
      - name: Build all channel manifests
        run: |
          # Build each channel to its own directory
          for channel in stable beta dev; do
            hugo --minify \
              --destination "public/releases/${channel}" \
              --params "channel=${channel}"
          done
          
          # Copy any static assets
          cp -r static/* public/ 2>/dev/null || true
      
      - name: Deploy to Cloudflare Pages
        uses: cloudflare/pages-action@v1
        with:
          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          accountId: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
          projectName: simtezilo-updates
          directory: public
```

### Required Secrets

Configure these secrets in your GitHub repository:

| Secret | Description |
|--------|-------------|
| `CLOUDFLARE_ACCOUNT_ID` | Your Cloudflare account ID |
| `CLOUDFLARE_API_TOKEN` | API token with Pages edit permissions |
| `R2_ACCESS_KEY_ID` | R2 API access key ID |
| `R2_SECRET_ACCESS_KEY` | R2 API secret access key |
| `UPDATES_REPO_TOKEN` | PAT with write access to updates repo |
| `CF_PAGES_DEPLOY_HOOK` | (Optional) Deploy hook URL for manual triggers |

## Expected Manifest Format

The manifest JSON file served at `https://simtezilo.com/releases/{channel}/latest.json`:

```json
{
  "version": "1.2.0",
  "releaseDate": "2025-01-15T10:00:00Z",
  "channel": "stable",
  "minUpgradeVersion": "1.0.0",
  "changelog": "## What's New\n\n- Feature 1\n- Feature 2",
  "platforms": {
    "linux-arm64": {
      "url": "https://static.simtezilo.com/releases/v1.2.0/simtezilo-linux-arm64",
      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "size": 15728640
    }
  }
}
```

## Device Configuration

On Simtezilo devices, configure the update settings in `simtezilo.conf`:

```toml
[app.updates]
enabled = true
manifestURL = "https://simtezilo.com/releases/stable/latest.json"
channel = "stable"
checkIntervalMinutes = 60
autoInstall = false
```

### Channel URLs

| Channel | Manifest URL |
|---------|-------------|
| Stable | `https://simtezilo.com/releases/stable/latest.json` |
| Beta | `https://simtezilo.com/releases/beta/latest.json` |
| Dev | `https://simtezilo.com/releases/dev/latest.json` |

### URL Structure

- **Manifests**: `https://simtezilo.com/releases/{channel}/latest.json`
- **Binaries**: `https://static.simtezilo.com/releases/{version}/simtezilo-linux-arm64`

## Security Considerations

1. **HTTPS Required**: Always serve manifests and binaries over HTTPS
2. **SHA256 Verification**: All downloads are verified against the SHA256 hash in the manifest
3. **Version Validation**: The `minUpgradeVersion` field prevents downgrades that might cause issues
4. **Signed Binaries** (optional): Consider GPG signing releases for additional security

## Rollback Support

The update system automatically tracks the previous version and supports rollback:

- Previous binary is preserved in the data directory
- If the service fails to start 3 times consecutively, auto-rollback is triggered
- Manual rollback can be triggered via the WebUI (future feature)

## Troubleshooting

### Update Check Fails

1. Verify network connectivity to update server
2. Check manifest URL is correct in config
3. Verify SSL certificate is valid
4. Check device logs for detailed error messages

### Download Fails

1. Check available disk space
2. Verify SHA256 matches between manifest and downloaded file
3. Check for network interruptions during download

### Installation Fails

1. Verify systemd service is correctly configured
2. Check file permissions on install directory
3. Review system logs: `journalctl -u simtezilo`
