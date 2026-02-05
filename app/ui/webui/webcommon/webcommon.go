// Package webcommon contains web UI assets shared between run mode and setup mode
package webcommon

import (
	"embed"
)

//go:embed static/*
var StaticFiles embed.FS
