//go:build !dev

package main

import "embed"

//go:embed templates static
var embeddedFS embed.FS

// contentFS is the filesystem used to serve templates and static files.
// In production builds, files are embedded in the binary.
var contentFS = embeddedFS
