//go:build dev

package server

import (
	"io/fs"
	"os"
)

// ContentFS is the filesystem used to serve templates and static files.
// In dev builds (-tags dev), files are read from disk for hot reload.
var ContentFS fs.FS = os.DirFS("internal/server")
