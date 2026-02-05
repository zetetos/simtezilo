# Development

## Required Tools

* **Golang** - See `go.mod` for the required version (currently 1.25.6)
* **golangci-lint** - Linter aggregator for Go
* **git** - Version control
* **make** - Build automation
* **Docker** - Required for cross-platform builds (Linux ARM/AMD64)

### Standard Linux/BSD/GNU Tools
* touch, cp, mv, rm
* cat, sed, grep, awk
* date, uname

### Additional Tools for Releases

* **git-cliff** - Changelog generation from conventional commits
* **sha** or **sha256sum** - Checksum generation for release artifacts
* **rclone** - Cloud storage sync for publishing releases to Cloudflare R2

### Optional Development Tools

* **air** - Live reload for development (installed automatically via `make run/watch`)
* **gotestsum** - Enhanced test output formatting (installed automatically via `make test`)

---

## Getting Started

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/zetetos/simtezilo.git
cd simtezilo

# Build for your local platform
make build

# Run the application
make run
```

### View Available Make Targets

```bash
make help
```

---

## Code Style & Conventions

Generally the styles and conventions are automatically enforced via the enabled linting rules.

### Formatting

* Use `gofmt` for general formatting
* The project uses additional formatters configured in `.golangci.yml`:
  - `gci` - Import grouping
  - `gofumpt` - Stricter gofmt
  - `goimports` - Import management
* Line length limit: **180 characters**

### Error Handling

* **Never use inline error handling** - Always check errors explicitly
* Errors should be handled on separate lines for clarity

### JSON Tags

* Use **camelCase** for JSON struct tags (enforced by `tagliatelle` linter)

---

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with watch mode (auto-rerun on file changes)
make test/watch

# Run tests with coverage report
make test/cover
```

### Writing Tests

* Use **testify** for test assertions (`github.com/stretchr/testify`)
* Arrange tests in **Arrange, Act, Assert** sections
* Name tests using the pattern: `Test[FeatureBeingTested]`

Example:
```go
func TestCalculateFuelConsumption(t *testing.T) {
    // Arrange
    lap := NewLap(...)
    
    // Act
    consumption := lap.CalculateFuelConsumption()
    
    // Assert
    assert.Equal(t, expected, consumption)
}
```

---

## Linting

```bash
# Run linter (reports issues)
make lint

# Run linter with auto-fix
make lint/fix
```

All new changes **must pass all linter checks**. The project uses `golangci-lint` with a comprehensive configuration in `.golangci.yml`.

### Key Linter Settings

* Most linters are enabled by default with specific exclusions
* Import restrictions enforced via `depguard` - only approved packages allowed
* See `.golangci.yml` for the full configuration

---

## Building

### Local Development Build

```bash
make build          # Build for current platform
make run            # Run with info logging
make run/debug      # Run with debug logging
make run/watch      # Run with live reload on file changes
```

### Cross-Platform Builds

```bash
# macOS Apple Silicon (not cross-compiled, requires Appl Silicon host)
make build/darwin/silicon

# Windows 64-bit (requires mingw-w64 cross-compiler)
make build/windows/64

# Linux AMD64 (via Docker)
make build/linux/amd64

# Raspberry Pi variants (via Docker)
make build/rpi/v6        # ARMv6 (Pi 1, Zero)
make build/rpi/v7        # ARMv7 (Pi 2B)
make build/rpi/v8/32     # ARMv8 32-bit (Pi 3, 4, 5, Zero 2W)
make build/rpi/v8/64     # ARMv8 64-bit (Pi 3, 4, 5, Zero 2W)
```

---

## Profiling

The project integrates with [Pyroscope](https://pyroscope.io/) for continuous profiling.

```bash
# Start Pyroscope container
make start-pyroscope

# Run application with profiling enabled
make run/profile

# Stop Pyroscope container
make stop-pyroscope
```

Access Pyroscope UI at: http://localhost:4040

Alternatively, use Docker Compose:
```bash
docker-compose up -d pyroscope
```

---

## Version Management

The project uses semantic versioning with the version stored in the `VERSION` file. See [Version Management](version_management.md) for full documentation.

Quick reference:
```bash
make version/show    # Display current version
make version/check   # Analyze commits and show recommended bump (dry-run)
make version/auto    # Automatically determine and apply version bump
make version/tag     # Create git tag from VERSION
```

---

## Releases

See [Release Process](release_process.md) for full documentation on creating and publishing releases.

Quick reference:
```bash
make release                            # Build all platforms and generate manifest
R2_REMOTE=r2:mybucket make release/publish  # Upload to Cloudflare R2
```

---

## Quality Assurance

```bash
# Run full audit (version validation, tests, formatting, vetting, vulnerability scan)
make audit

# Check for dependency upgrades
make upgradeable
```

---

## Cleanup

```bash
make clean       # Remove build outputs (out/ directory)
make distclean   # Full cleanup (out/, dist/, profiler data, Docker cache)
```

---

## Project Structure

```
simtezilo/
├── api/            # API definitions
├── app/            # Core application logic
│   ├── cache/      # Caching utilities
│   ├── calibrator/ # Calibration logic
│   ├── circuit/    # Circuit/track handling
│   ├── codec/      # Audio/video codecs
│   ├── config/     # Configuration management
│   ├── haptics/    # Haptic feedback system
│   ├── hardware/   # Hardware interfaces
│   ├── i18n/       # Internationalization
│   ├── pitradio/   # Pit radio/Discord integration
│   └── ...
├── build/          # Build scripts and Dockerfiles
│   ├── docker/     # Docker build configurations
│   └── scripts/    # Build and release scripts
├── circuits/       # Circuit data files
├── cmd/            # Application entry points
│   ├── simtezilo/  # Main application
│   ├── platform-m1/# Platform-specific tools
│   └── wifi/       # WiFi utilities
├── data/           # Runtime data and assets
├── doc/            # Documentation
├── init/           # System service files
└── tools/          # Development tools
```

---

## Dependencies

### Key Dependencies

* **gt-telemetry** - Gran Turismo telemetry parsing (`github.com/zetetos/gt-telemetry/v2`)
* **viper** - Configuration management
* **zerolog** - Structured logging
* **testify** - Test assertions
* **beep** - Audio playback
* **discordgo** - Discord bot integration (using fork for voice fixes)

### Working with Local Dependencies

To develop against a local copy of `gt-telemetry`, uncomment the replace directive in `go.mod`:

```go
replace github.com/zetetos/gt-telemetry/v2 => /path/to/local/gt-telemetry
```

---

## Commit Message Format

This project follows [Conventional Commits](https://www.conventionalcommits.org/). See [Version Management](version_management.md) for commit format details and how commit types affect version bumps.

---

## Troubleshooting

### Windows Cross-Compilation

Building for Windows requires `mingw-w64`:

```bash
# macOS
brew install mingw-w64

# Ubuntu/Debian
apt-get install gcc-mingw-w64-x86-64
```

### Docker Build Issues

Ensure Docker is running and has sufficient resources allocated. For ARM builds, Docker buildx with QEMU emulation is used.

### Linter Failures

If `make lint` fails, try `make lint/fix` first to auto-fix issues. Some linters are currently disabled with TODOs - see `.golangci.yml` for details.