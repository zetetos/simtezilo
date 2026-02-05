# Version Management

The project version is managed via the `VERSION` file at the project root.

## Quick Start

### Automatic Version Bumping (Recommended)

Use conventional commits and let the system determine the version:

```bash
# Make your commits using conventional commit format
git commit -m "feat: add new feature"
git commit -m "fix: resolve bug"

# Automatically determine and apply the version bump
make version/auto

# Or just check what would be bumped (dry-run)
make version/check
```

### Manual Version Bumping

```bash
make version/bump-patch  # 0.8.0 -> 0.8.1
make version/bump-minor  # 0.8.0 -> 0.9.0
make version/bump-major  # 0.8.0 -> 1.0.0
```

### Creating Release Tags

```bash
make version/tag  # Creates a git tag matching VERSION file
git push --tags   # Push tags to remote
```

## Conventional Commits

The automatic version detection follows the [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Commit Format
```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Version Bump Rules

|          Commit Pattern        | Version Bump |              Example              |
|--------------------------------|--------------|-----------------------------------|
| `fix:`                         |  **Patch**   | `fix: resolve memory leak`        |
| `feat:`                        |  **Minor**   | `feat: add user authentication`   |
| `perf:`                        |  **Patch**   | `perf: optimize database queries` |
| `feat!:` or `BREAKING CHANGE:` |  **Major**   | `feat!: redesign API`             |

### Changelog Mapping

Commit types are mapped to changelog sections by **git-cliff** (configured in `cliff.toml`):

| Commit Type                  | Changelog Section |
|------------------------------|-------------------|
| `feat:`                      | Added             |
| `fix:`                       | Fixed             |
| `doc:`                       | Documentation     |
| `perf:`                      | Performance       |
| `refactor:`                  | Changed           |
| `style:`                     | Changed           |
| `chore:`                     | Changed           |
| `revert:`                    | Removed           |
| `test:`, `ci:`, `build:`     | Skipped           |

### Examples

**Patch bump (bug fixes):**
```bash
git commit -m "fix: correct typo in error message"
git commit -m "perf: reduce memory allocation"
```

**Minor bump (new features):**
```bash
git commit -m "feat: add export functionality"
git commit -m "feat(ui): implement dark mode"
```

**Major bump (breaking changes):**
```bash
git commit -m "feat!: redesign configuration API"
# or
git commit -m "feat: redesign configuration API

BREAKING CHANGE: config format has changed"
```

**No version bump:**
```bash
git commit -m "docs: update README"
git commit -m "chore: update dependencies"
git commit -m "refactor: simplify code structure"
git commit -m "test: add unit tests"
```

## How It Works

- **VERSION file**:      Single source of truth at project root
- **update_version.sh**: Shell script containing all version management logic
- **Makefile**:          Thin wrappers around `update_version.sh` for convenience
- **Build injection**:   Version injected into binary via `-ldflags -X app.Version=$(buildversion)`
- **Fallback**:          If VERSION file missing, falls back to "dev"
- **Validation**:        `make audit` now includes VERSION validation

## Direct Script Usage

You can also use the script directly without Make:

```bash
# Show help
./build/scripts/update_version.sh help

# Auto-bump (default when no args)
./build/scripts/update_version.sh
./build/scripts/update_version.sh auto

# Manual bumps
./build/scripts/update_version.sh patch
./build/scripts/update_version.sh minor
./build/scripts/update_version.sh major

# Other commands
./build/scripts/update_version.sh validate
./build/scripts/update_version.sh show
./build/scripts/update_version.sh check
./build/scripts/update_version.sh tag
```

## Makefile Targets

```bash
make version/show         # Display current version
make version/validate     # Validate VERSION file format
make version/check        # Show recommended bump (dry-run)
make version/auto         # Auto-bump based on commits
make version/bump-patch   # Manual patch bump
make version/bump-minor   # Manual minor bump
make version/bump-major   # Manual major bump
make version/tag          # Create git tag
```

## CI/CD Integration

### Option 1: Automatic Version Management (Recommended)

```yaml
# .github/workflows/release.yml
- name: Determine version
  run: make version/auto
  
- name: Create release tag
  run: make version/tag
  
- name: Build
  run: make dist
```

### Option 2: Manual Version File

```yaml
# Developer updates VERSION file manually
- name: Build with existing version
  run: make dist
```

### Option 3: Git-based Versioning

```yaml
- name: Generate version from git
  run: |
    git describe --tags > VERSION
    make dist
```

## Workflow Example

### Feature Development

```bash
# 1. Start with v0.8.0
git checkout -b feature/new-thing

# 2. Make commits using conventional format
git commit -m "feat: add new feature"
git commit -m "fix: resolve edge case"

# 3. Check what version would be bumped to
make version/check
# Output: Recommended: MINOR version bump

# 4. Apply the version bump
make version/auto
# Output: Version bumped: v0.8.0 -> v0.9.0

# 5. Create a tag and push
make version/tag
git push origin feature/new-thing --tags
```

### Hotfix Release

```bash
# 1. Create hotfix branch
git checkout -b hotfix/critical-bug

# 2. Fix the issue
git commit -m "fix: resolve critical security issue"

# 3. Bump version (will be patch: 0.8.0 -> 0.8.1)
make version/auto

# 4. Tag and release
make version/tag
git push --tags
```

## Best Practices

1. **Use conventional commits** for all changes
2. **Run `make version/check`** before finalizing a release
3. **Tag releases** using `make version/tag`
4. **Validate** with `make audit` before committing
5. **Never edit VERSION file directly** - use Makefile targets

## Benefits

- **Automated**:         Analyzes git history to determine appropriate version
- **Consistent**:        Follows semantic versioning automatically
- **Safe**:              Validates format and prevents errors
- **Flexible**:          Works in CI/CD and local development
- **Transparent**:       Shows reasoning for version decisions
- **No external tools**: Pure shell script, no dependencies
