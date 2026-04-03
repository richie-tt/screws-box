//go:build !dev

package server

import "embed"

//go:embed templates static
var embeddedFS embed.FS

// ContentFS is the filesystem used to serve templates and static files.
// In production builds, files are embedded in the binary.
var ContentFS = embeddedFS
