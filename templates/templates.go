// Package templates holds the page templates, embedded into the binary.
//
// The package lives at the repo root (not inside internal/build) so the
// template files stay at the layout documented in docs/BUILD_PLAN.md:
// //go:embed can only reach files inside its own package directory.
package templates

import "embed"

// Files is the embedded template filesystem, rooted at this directory.
//
//go:embed index.gohtml
var Files embed.FS
