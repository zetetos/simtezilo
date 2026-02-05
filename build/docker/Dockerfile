# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25.6

FROM golang:${GO_VERSION}-trixie AS base
WORKDIR /src
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x



FROM base AS base-armhf
RUN dpkg --add-architecture armhf
RUN apt-get update && apt-get install -y libasound2-dev:armhf gcc-arm-linux-gnueabihf libopus-dev

FROM base AS base-armel
RUN dpkg --add-architecture armel
RUN apt-get update && apt-get install -y libasound2-dev:armel gcc-arm-linux-gnueabi libopus-dev:armel

FROM base AS base-arm64
RUN dpkg --add-architecture arm64
RUN apt-get update && apt-get install -y libasound2-dev:arm64 gcc:arm64 libopus-dev:arm64

FROM base AS base-amd64
RUN dpkg --add-architecture amd64
RUN apt-get update && apt-get install -y libasound2-dev:amd64 gcc:amd64 libopus-dev:amd64


FROM base-armhf AS build-app-armhf
WORKDIR /src
ARG CGO_ENABLED=1
ARG CC=arm-linux-gnueabihf-gcc
ARG PKG_CONFIG_PATH=/usr/lib/arm-linux-gnueabihf/pkgconfig
ARG GOOS
ARG BUILDMODULE
ARG BUILDVERSION
ARG BUILDCOMMIT
ARG BUILDTIME
ARG PLATFORM
RUN --mount=type=bind,target=. find /src -type f
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/simtezilo-${GOOS}-armhf ./cmd/simtezilo/main.go
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/platform-m1-${GOOS}-armhf ./cmd/platform-m1/*.go

FROM base-armel AS build-app-armel
WORKDIR /src
ARG CGO_ENABLED=1
ARG CC=arm-linux-gnueabi-gcc
ARG PKG_CONFIG_PATH=/usr/lib/arm-linux-gnueabi/pkgconfig
ARG GOOS
ARG GOARM
ARG BUILDMODULE
ARG BUILDVERSION
ARG BUILDCOMMIT
ARG BUILDTIME
ARG PLATFORM
RUN --mount=type=bind,target=. find /src -type f
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm GOARM=${GOARM} \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/simtezilo-${GOOS}-armel-${GOARM} ./cmd/simtezilo/main.go
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm GOARM=${GOARM} \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/platform-m1-${GOOS}-armel-${GOARM} ./cmd/platform-m1/*.go

FROM base-armel AS build-app-armel-8
WORKDIR /src
ARG CGO_ENABLED=1
ARG CC=arm-linux-gnueabi-gcc
ARG PKG_CONFIG_PATH=/usr/lib/arm-linux-gnueabi/pkgconfig
ARG GOOS
ARG BUILDMODULE
ARG BUILDVERSION
ARG BUILDCOMMIT
ARG BUILDTIME
ARG PLATFORM
RUN --mount=type=bind,target=. find /src -type f
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/simtezilo-${GOOS}-armel-8 ./cmd/simtezilo/main.go
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/platform-m1-${GOOS}-armel-8 ./cmd/platform-m1/*.go

FROM base-arm64 AS build-app-arm64-8
WORKDIR /src
ARG CGO_ENABLED=1
ARG GOOS
ARG BUILDMODULE
ARG BUILDVERSION
ARG BUILDCOMMIT
ARG BUILDTIME
ARG PLATFORM
RUN --mount=type=bind,target=. find /src -type f
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm64 \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/simtezilo-${GOOS}-arm64-8 ./cmd/simtezilo/main.go
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=arm64 \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/platform-m1-${GOOS}-arm64-8 ./cmd/platform-m1/*.go

FROM base-amd64 AS build-app-amd64
WORKDIR /src
ARG CGO_ENABLED=1
ARG PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig
ARG GOOS
ARG BUILDMODULE
ARG BUILDVERSION
ARG BUILDCOMMIT
ARG BUILDTIME
ARG PLATFORM
RUN --mount=type=bind,target=. find /src -type f
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    GOOS=${GOOS} GOARCH=amd64 \
    go build -ldflags "-X '${BUILDMODULE}/app.Version=${BUILDVERSION}' -X '${BUILDMODULE}/app.CommitHash=${BUILDCOMMIT}' -X '${BUILDMODULE}/app.BuildTime=${BUILDTIME}' -X '${BUILDMODULE}/app.Platform=${PLATFORM}'" -o /out/simtezilo-${GOOS}-amd64 ./cmd/simtezilo/main.go


FROM scratch AS binaries-armhf
ARG GOOS
ARG GOARM
COPY --from=build-app-armhf /out/simtezilo-${GOOS}-armhf /
COPY --from=build-app-armhf /out/platform-m1-${GOOS}-armhf /

FROM scratch AS binaries-armel
ARG GOOS
ARG GOARM
COPY --from=build-app-armel /out/simtezilo-${GOOS}-armel-${GOARM} /
COPY --from=build-app-armel /out/platform-m1-${GOOS}-armel-${GOARM} /

FROM scratch AS binaries-armel-8
ARG GOOS
COPY --from=build-app-armel-8 /out/simtezilo-${GOOS}-armel-8 /
COPY --from=build-app-armel-8 /out/platform-m1-${GOOS}-armel-8 /  

FROM scratch AS binaries-arm64-8
ARG GOOS
COPY --from=build-app-arm64-8 /out/simtezilo-${GOOS}-arm64-8 /
COPY --from=build-app-arm64-8 /out/platform-m1-${GOOS}-arm64-8 /

FROM scratch AS binaries-amd64
ARG GOOS
COPY --from=build-app-amd64 /out/simtezilo-${GOOS}-amd64 /