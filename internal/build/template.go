// Package build implements the posterpage build pipeline: load + validate the
// config (milestone 1), resolve styles, render the page (with head-tag
// injection), and copy assets (milestones 2–4). See docs/BUILD_PLAN.md.
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

// Input is the render input: the resolved config plus the build-stage data
// the template can't compute itself. HasStyles is set once the styles step
// has produced styles.css — always, in practice, since config validation
// guarantees a styles source — which turns on the stylesheet link in <head>.
// HeadTags is the emit-ready meta/link/script list flattened from the config
// by headTags (fonts preconnects + stylesheet first, then head meta, links,
// scripts); NavLinks is the nav entries with their render style resolved, so
// the template never touches style inheritance or tag construction.
type Input struct {
	*config.Config
	HasStyles bool
	HeadTags  []headTag
	NavLinks  []NavLink
}

// NavLink is one nav entry resolved for rendering: the config link's text,
// href and subtitle plus its effective render style. Effective style is the
// entry's own style when set, else site.link_style — per-entry wins, so
// mixed lists are legal. IsButton selects the button markup in the template;
// anything else renders the plain anchor.
type NavLink struct {
	Text     string
	Href     string
	Subtitle string
	IsButton bool
}

// navLinks resolves cfg.Links against site.link_style. Both values are
// validated to button | link (Load defaults site.link_style to button; the
// same default is applied again here for configs built without Load).
// Anything other than the plain-anchor style renders as a button.
func navLinks(cfg *config.Config) []NavLink {
	links := make([]NavLink, len(cfg.Links))
	for i, l := range cfg.Links {
		style := l.Style
		if style == "" {
			style = cfg.Site.LinkStyle
		}
		if style == "" {
			style = "button"
		}
		links[i] = NavLink{
			Text:     l.Text,
			Href:     l.Href,
			Subtitle: l.Subtitle,
			IsButton: style != "link",
		}
	}
	return links
}

// indexTmpl is the parsed page template.
var indexTmpl = template.Must(template.ParseFS(templates.Files, "index.gohtml"))

// RenderIndex writes the page HTML to w.
func RenderIndex(w io.Writer, in *Input) error {
	return indexTmpl.Execute(w, in)
}

// writeIndex renders the page into <outDir>/index.html and returns its byte
// size. HasStyles is always true here: the styles step has already run (and
// produced <outDir>/styles.css) by the time render happens; the head tags and
// nav links are flattened from the config right before rendering.
func writeIndex(outDir string, cfg *config.Config) (int64, error) {
	var buf bytes.Buffer
	in := &Input{Config: cfg, HasStyles: true, HeadTags: headTags(cfg), NavLinks: navLinks(cfg)}
	if err := RenderIndex(&buf, in); err != nil {
		return 0, err
	}
	path := filepath.Join(outDir, "index.html")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}
	return int64(buf.Len()), nil
}
