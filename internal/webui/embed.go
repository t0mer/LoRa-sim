// Package webui embeds the built React SPA so the single Cylon binary serves the
// web app. The Vite build writes to ./dist; a tracked .gitkeep keeps the embed
// compiling before any frontend build has run.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the SPA file system rooted at the build output directory.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// HasIndex reports whether a built SPA (index.html) is present, vs. only the
// placeholder.
func HasIndex() bool {
	_, err := fs.Stat(dist, "dist/index.html")
	return err == nil
}
