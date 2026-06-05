// Package skilldata embeds the 54 built-in skill YAML files at compile time.
package skilldata

import "embed"

// FS provides read-only access to the built-in skills/ directory.
//
//go:embed skills
var FS embed.FS
