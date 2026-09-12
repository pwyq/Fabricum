// Package frontend embeds the editor served by the local processor.
package frontend

import "embed"

// Files contains the editor's static assets.
//
//go:embed static
var Files embed.FS
