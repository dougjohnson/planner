package main

import (
	"embed"
	"io/fs"
)

// frontendDist embeds the built frontend assets.
//
// For production builds, the Makefile copies frontend/dist/ into
// cmd/flywheel-planner/dist/ before compilation so the embed path is valid
// (Go's embed directive does not support ".." in paths).
//
// For development, this directory may not exist — the devFrontendFS()
// fallback provides an empty FS so the binary still compiles.

//go:embed all:dist
var frontendDist embed.FS

// frontendDistFS returns the embedded frontend filesystem.
// Falls back to an empty FS if the dist directory was not embedded
// (e.g., during development without a prior frontend build).
func frontendDistFS() fs.FS {
	sub, err := fs.Sub(frontendDist, "dist")
	if err != nil {
		// dist/ not embedded — return empty FS for dev mode.
		return emptyFS{}
	}
	return sub
}

// emptyFS implements fs.FS with no files — used as fallback in dev mode.
type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: "/", Err: fs.ErrNotExist}
}
