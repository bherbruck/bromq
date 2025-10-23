package web

import (
	"embed"
	"io/fs"
)

// Files starting with . and _ are excluded by default
//
//go:embed all:dist/client
var assets embed.FS

// FS contains the web UI assets with the dist/client prefix stripped.
var FS, _ = fs.Sub(assets, "dist/client")
