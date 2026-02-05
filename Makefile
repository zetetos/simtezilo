# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## audit: run quality control checks
.PHONY: audit
audit: version/validate test
	go mod tidy -diff
	go mod verify
	test -z "$(shell gofmt -l .)" 
	go vet ./...
#   waiting for fix: https://github.com/dominikh/go-tools/issues/1653
# 	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## test: run all tests
.PHONY: test
test:
	@go run gotest.tools/gotestsum@latest \
		--format testname  \
		--format-hide-empty-pkg \
		-- -race -buildvcs ./...
	
## test/watch: run all tests re-run when any files change
.PHONY: test/watch
test/watch:
	@go run gotest.tools/gotestsum@latest \
		--format pkgname-and-test-fails \
		--format-icons hivis \
		--format-hide-empty-pkg \
		--watch \
	 	-- -v -race -buildvcs ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	mkdir -p out
	go test -v -race -buildvcs -coverprofile=out/coverage.out ./...
	go tool cover -html=out/coverage.out

## upgradeable: list direct dependencies that have upgrades available
.PHONY: upgradeable
upgradeable:
	@go run github.com/oligot/go-mod-upgrade@latest


# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

buildmodule := $(shell awk '/module/ {print $$NF}' go.mod)
buildversion := $(shell cat VERSION 2>/dev/null || echo "v0.0.1-dev.1")
buildcommit := $(shell git describe --tags --always --dirty)
buildtime := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
platform := $(shell uname -s | tr '[:upper:]' '[:lower:]')

## lint: run linter against project
.PHONY: lint
lint:
	@golangci-lint run --max-issues-per-linter 0 --max-same-issues 0

## lint/fix: run linter against the project and fix issues where possible
.PHONY: lint/fix
lint/fix:
	@golangci-lint run --fix

## dist: create distribution archives in dist/releases/<channel>/v<version>/
.PHONY: dist
dist: build/rpi/v8/64 build/linux/amd64 build/windows/64 build/darwin/silicon
	@chmod +x ./build/scripts/gen_dist.sh
	@./build/scripts/gen_dist.sh

## release/manifest: generate release manifest JSON (run after dist)
.PHONY: release/manifest
release/manifest:
	@chmod +x ./build/scripts/gen_release_manifest.sh
	@./build/scripts/gen_release_manifest.sh

## release: build all platforms, create archives, and generate manifest
.PHONY: release
release: dist release/manifest
	@echo ""
	@echo "Release ready at: dist/releases/stable/"
	@echo "To publish: make release/publish"

## release/publish: upload release artifacts to Cloudflare R2
# R2_REMOTE must be set to the rclone remote name for the R2 bucket
# e.g., "R2_REMOTE=r2:mybucket make release/publish"
.PHONY: release/publish
release/publish:
	@if [ -z "$(R2_REMOTE)" ]; then echo "Error: R2_REMOTE is not set"; exit 1; fi
	@echo "Uploading release to Cloudflare R2..."
	@rclone sync dist/releases/ $(R2_REMOTE)/releases/ \
		--exclude ".DS_Store" \
		--progress
	@echo "Release published to R2 remote: $(R2_REMOTE)"

## version/validate: validate VERSION file format
.PHONY: version/validate
version/validate:
	@./build/scripts/update_version.sh validate

## version/show: display current version
.PHONY: version/show
version/show:
	@./build/scripts/update_version.sh show

## version/patch: increment patch version (0.8.0 -> 0.8.1)
.PHONY: version/patch
version/patch:
	@./build/scripts/update_version.sh patch

## version/minor: increment minor version (0.8.0 -> 0.9.0)
.PHONY: version/minor
version/minor:
	@./build/scripts/update_version.sh minor

## version/major: increment major version (0.8.0 -> 1.0.0)
.PHONY: version/major
version/bump-major:
	@./build/scripts/update_version.sh major

## version/prerelease: add or bump pre-release version (0.8.0 -> 0.8.0-beta.1)
.PHONY: version/prerelease
version/prerelease:
	@./build/scripts/update_version.sh prerelease beta

## version/release: remove pre-release suffix for stable release
.PHONY: version/release
version/release:
	@./build/scripts/update_version.sh release

## version/auto: automatically determine and apply version bump from conventional commits
.PHONY: version/auto
version/auto:
	@./build/scripts/update_version.sh auto

## version/check: analyze commits and show recommended version bump (dry-run)
.PHONY: version/check
version/check:
	@./build/scripts/update_version.sh check

## version/tag: create a git tag matching the VERSION file
.PHONY: version/tag
version/tag:
	@./build/scripts/update_version.sh tag

## build: build the application for the current platform
.PHONY: build
build:
	go build -ldflags "X '$(buildmodule)/app.Version=$(buildversion)' -X '$(buildmodule)/app.CommitHash=$(buildcommit)' -X '$(buildmodule)/app.BuildTime=$(buildtime)' -X '$(buildmodule)/app.Platform=local'" \
	-o ./out/simtezilo-local ./cmd/simtezilo/main.go
	go build -ldflags "-X '$(buildmodule)/app.Version=$(buildversion)' -X '$(buildmodule)/app.CommitHash=$(buildcommit)' -X '$(buildmodule)/app.BuildTime=$(buildtime)' -X '$(buildmodule)/app.Platform=local'" \
	-o ./out/platform-local ./cmd/platform-m1/*.go

## build/darwin/silicon: build the application for Apple Silicon
.PHONY: build/darwin/silicon
build/darwin/silicon:
	GOOS=darwin GOARCH=arm64 \
	go build -ldflags "-X '$(buildmodule)/app.Version=$(buildversion)' -X '$(buildmodule)/app.CommitHash=$(buildcommit)' -X '$(buildmodule)/app.BuildTime=$(buildtime)' -X '$(buildmodule)/app.Platform=darwin'" \
	-o ./out/simtezilo-macos ./cmd/simtezilo/main.go

## build/windows/amd64: build the application for Windows 64-bit
.PHONY: build/windows/amd64
build/windows/64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
	go build -ldflags "-X '$(buildmodule)/app.Version=$(buildversion)' -X '$(buildmodule)/app.CommitHash=$(buildcommit)' -X '$(buildmodule)/app.BuildTime=$(buildtime)' -X '$(buildmodule)/app.Platform=windows' -s" \
	-o ./out/simtezilo.exe ./cmd/simtezilo/main.go

## build/linux/amd64: build the application for Linux 64-bit
.PHONY: build/linux/amd64
build/linux/amd64:
	@docker build \
	--build-arg GOOS=linux --build-arg GOARCH=amd64 \
	--build-arg BUILDMODULE=$(buildmodule) \
	--build-arg BUILDTIME=$(buildtime) \
	--build-arg BUILDVERSION=$(buildversion) \
	--build-arg BUILDCOMMIT=$(buildcommit) \
	--build-arg PLATFORM="simtezilo" \
	--output=out --target=binaries-amd64 --progress=plain \
	-f build/docker/Dockerfile .

## build/rpi: build the application for Raspberry Pi using ARMHF (any version)
.PHONY: build/rpi
build/rpi:
	@docker build \
	--build-arg GOOS=linux --build-arg GOARCH=arm \
	--build-arg BUILDMODULE=$(buildmodule) \
	--build-arg BUILDTIME=$(buildtime) \
	--build-arg BUILDVERSION=$(buildversion) \
	--build-arg BUILDCOMMIT=$(buildcommit) \
	--build-arg PLATFORM="simtezilo" \
	--output=out --target=binaries-armhf --progress=plain \
	-f build/docker/Dockerfile .

## build/rpi/v6: build the application for Raspberry Pi ARMv6 (1*, Zero*)
.PHONY: build/rpi/v6
build/rpi/v6:
	@docker build \
	--build-arg GOOS=linux --build-arg GOARCH=arm --build-arg GOARM=6 \
	--build-arg BUILDMODULE=$(buildmodule) \
	--build-arg BUILDTIME=$(buildtime) \
	--build-arg BUILDVERSION=$(buildversion) \
	--build-arg BUILDCOMMIT=$(buildcommit) \
	--build-arg PLATFORM="simtezilo" \
	--output=out --target=binaries-armel --progress=plain \
	-f build/docker/Dockerfile .

## build/rpi/v7: build the application for Raspberry Pi ARMv7 (2B)
.PHONY: build/rpi/v7
build/rpi/v7:
	@docker build \
	--build-arg GOOS=linux --build-arg GOARM=7 \
	--build-arg BUILDMODULE=$(buildmodule) \
	--build-arg BUILDTIME=$(buildtime) \
	--build-arg BUILDVERSION=$(buildversion) \
	--build-arg BUILDCOMMIT=$(buildcommit) \
	--build-arg PLATFORM="simtezilo" \
	--output=out --target=binaries-armel --progress=plain \
	-f build/docker/Dockerfile .

## build/rpi/v8/32: build the application for Raspberry Pi ARMv8 32bit (2B[+1.2], 3*, 4*, 5*, Zero 2W)
.PHONY: build/rpi/v8/32
build/rpi/v8/32:
	@docker build \
	--build-arg GOOS=linux \
	--build-arg BUILDMODULE=$(buildmodule) \
	--build-arg BUILDTIME=$(buildtime) \
	--build-arg BUILDVERSION=$(buildversion) \
	--build-arg BUILDCOMMIT=$(buildcommit) \
	--build-arg PLATFORM="simtezilo" \
	--output=out --target=binaries-armel-8 --progress=plain \
	-f build/docker/Dockerfile .

## build/rpi/v8/64: build the application for Raspberry Pi ARMv8 64bit (3*, 4*, 5*, Zero 2W)
.PHONY: build/rpi/v8/64
build/rpi/v8/64:
	@docker build \
	--build-arg GOOS=linux --build-arg GOARCH=arm64 \
	--build-arg BUILDMODULE=$(buildmodule) \
	--build-arg BUILDTIME=$(buildtime) \
	--build-arg BUILDVERSION=$(buildversion) \
	--build-arg BUILDCOMMIT=$(buildcommit) \
	--build-arg PLATFORM="simtezilo" \
	--output=out --target=binaries-arm64-8 --progress=plain \
	-f build/docker/Dockerfile .

## run: run the application locally
.PHONY: run
run:
	@go run cmd/simtezilo/main.go -l info

## run/debug: run the application locally with denug logging enabled
.PHONY: run/debug
run/debug:
	@go run cmd/simtezilo/main.go -l debug -w=true

## run/watch: run the application locally and reload on file changes
.PHONY: run/watch
run/watch:
	go run github.com/air-verse/air@latest \
		--build.cmd "make build" --build.bin "out/simtezilo-local" --build.args_bin "-w" --build.delay "100" \
		--build.include_dir "app, cmd" \
		--build.include_ext "go, html, js, png, svg" \
		--build.include_file "simtezilo.conf" \
		--build.send_interrupt "true" \
		--misc.clean_on_exit "true"

## run/profile: run the application with profiling enabled
.PHONY: run/profile
run/profile:
	@go run cmd/simtezilo/main.go -l info -p=http://localhost:4040 -w=true

## start-pyroscope: start the Pyroscope profiler Docker container
.PHONY: start-pyroscope
start-pyroscope:
	@docker run \
	--name pyroscope \
	--rm --detach \
	-p 4040:4040 \
	-v $(shell pwd)/data/persist/pyroscope:/data \
	grafana/pyroscope:latest
	@echo "Pyroscope started. Access it at http://localhost:4040"

## stop-pyroscope: stop the Pyroscope profiler Docker container
.PHONY: stop-pyroscope
stop-pyroscope:
	@docker stop pyroscope

## clean: clean up build output files
.PHONY: clean
clean:
	@rm -rf out
	@go clean -cache

## distclean: clean up project and return to a pristine state
.PHONY: distclean
distclean: clean
	@rm -rf dist
	@rm -rf data/persist/pyroscope
	@docker builder prune -af