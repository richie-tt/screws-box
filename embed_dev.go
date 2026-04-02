//go:build dev

package main

import (
	"io/fs"
	"os"
)

// contentFS is the filesystem used to serve templates and static files.
// In dev builds (-tags dev), files are read from disk for hot reload.
var contentFS fs.FS = os.DirFS(".")
