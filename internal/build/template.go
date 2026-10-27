// Package build implements the posterpage build pipeline: load + validate the
// config (milestone 1), render the page and copy assets (milestone 2). The
// styles step (sass / css passthrough) lands in milestone 3, head-tag
// injection in milestone 4 — see docs/BUILD_PLAN.md.
package build

import (
	"bytes"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"posterpage/internal/config"
	"posterpage/templates"
)

// Input is the render input: the resolved config plus build-stage flags.
// HasStyles is set once the styles step (milestone 3) has produced
// styles.css, which turns on the stylesheet link in <head>.
type Input struct {
	*config.Config
	HasStyles bool
}

// indexTmpl is the parsed page template.
var indexTmpl = template.Must(template.ParseFS(templates.Files, "index.gohtml"))

// RenderIndex writes the page HTML to w.
func RenderIndex(w io.Writer, in *Input) error {
	return indexTmpl.Execute(w, in)
}

// writeIndex renders the page (Input has HasStyles=false until milestone 3)
// into <outDir>/index.html and returns its byte size.
func writeIndex(outDir string, cfg *config.Config) (int64, error) {
	var buf bytes.Buffer
	if err := RenderIndex(&buf, &Input{Config: cfg}); err != nil {
		return 0, err
	}
	path := filepath.Join(outDir, "index.html")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}
	return int64(buf.Len()), nil
}
