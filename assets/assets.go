// Package assets holds the vendored Bootstrap 5.3.x Sass sources, embedded
// into the binary so an installed posterpage works without the source
// checkout nearby.
//
// The package lives at the repo root (not inside internal/build) so the
// vendored files stay at the layout documented in docs/BUILD_PLAN.md:
// //go:embed can only reach files inside its own package directory.
package assets

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// Bootstrap is the embedded vendored Bootstrap 5.3.x scss tree (the
// --load-path sources for the styles pipeline), rooted at this directory.
// all: also includes entries starting with _ or . (the vendored partials
// start with _); the tree is embedded unmodified.
//
//go:embed all:bootstrap
var Bootstrap embed.FS

// Extract writes the embedded Bootstrap tree to dest, preserving the
// bootstrap/ prefix (dest/bootstrap/_functions.scss, …). The styles step
// passes --load-path=<dest>/bootstrap to sass. Callers own dest; it is
// meant to be a fresh temp dir per build run.
func Extract(dest string) error {
	return fs.WalkDir(Bootstrap, "bootstrap", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dest, path), 0o755)
		}
		data, err := Bootstrap.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dest, path), data, 0o644)
	})
}
