# License management

## Generate License Report

```bash
go run github.com/google/go-licenses@latest report ./... 2>/dev/null | sort -t',' -k3
```

## Update LICENSES Directory

When dependencies change, update the `LICENSES/` directory with full license texts:

```bash
# Remove old licenses and regenerate
rm -rf /tmp/simtezilo-licenses
go run github.com/google/go-licenses@latest save ./... \
    --save_path=/tmp/simtezilo-licenses \
    --ignore github.com/zetetos/simtezilo \
    --ignore github.com/golang/freetype

# Copy to project (freetype is dual-licensed, added manually)
rm -rf LICENSES && cp -R /tmp/simtezilo-licenses LICENSES
mkdir -p LICENSES/github.com/golang/freetype
curl -sL https://raw.githubusercontent.com/golang/freetype/master/LICENSE \
    -o LICENSES/github.com/golang/freetype/LICENSE
```

## Check for Incompatible Licenses

Before adding new dependencies, verify GPL 3.0 compatibility:

```bash
go run github.com/google/go-licenses@latest check ./... --disallowed_types=forbidden
```

**Licenses incompatible with GPL 3.0 (do not use):**
- GPL 2.0 only (without "or later")
- LGPL 2.0 only (without "or later")  
- CDDL
- EPL 1.0
- MS-PL / MS-RL
- Any proprietary/commercial license

**Licenses requiring caution:**
- LGPL (must allow relinking; usually fine for dynamic linking in Go)
- MPL 2.0 (file-level copyleft; compatible but requires file separation)

## Maintainer Checklist

When adding or updating dependencies:

1. [ ] Run `go run github.com/google/go-licenses@latest check ./...` to verify compatibility
2. [ ] Regenerate the license report and review new entries
3. [ ] Update `LICENSES/` directory with new license files
4. [ ] Update `THIRD_PARTY_NOTICES` if license categories change
5. [ ] For dual-licensed dependencies, document which license option is chosen
6. [ ] Verify transitive dependencies don't introduce incompatible licenses

## Periodic Maintenance

Recommended quarterly or before major releases:

1. Run `go mod tidy` to remove unused dependencies
2. Run `go get -u ./...` to update dependencies (review changelogs for license changes)
3. Regenerate this report and compare against previous version
4. Verify all license files in `LICENSES/` are still accurate
5. Check upstream repositories for license changes (rare but possible)
