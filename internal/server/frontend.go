package server

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend
var frontendFS embed.FS

// getFrontendFS returns the frontend filesystem, or nil if not available.
func getFrontendFS() fs.FS {
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		return nil
	}
	// Check if index.html exists to verify the frontend is built.
	f, err := sub.Open("index.html")
	if err != nil {
		return nil
	}
	f.Close()
	return sub
}
