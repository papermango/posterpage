package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// problem is one validation finding: a config location (path + optionally the
// source line) and a message.
type problem struct {
	line int // 0 → no line number (file-level checks, or JSON configs)
	path string
	msg  string
}

func (p problem) render(file string) string {
	var b strings.Builder
	b.WriteString(file)
	if p.line > 0 {
		fmt.Fprintf(&b, ":%d", p.line)
	}
	if p.path != "" {
		fmt.Fprintf(&b, ": %s", p.path)
	}
	fmt.Fprintf(&b, ": %s", p.msg)
	return b.String()
}

// Validate checks the config against the schema and allowlist rules in
// docs/BUILD_PLAN.md. Referenced files are checked against dir (the config
// directory). All problems are collected and returned as ONE error rendered
// as an indented list; nil means the config is valid.
func Validate(c *Config, dir, file string) error {
	ps := validate(c, dir)
	if len(ps) == 0 {
		return nil
	}
	lines := make([]string, len(ps))
	for i, p := range ps {
		lines[i] = "  " + p.render(file)
	}
	plural := "s"
	if len(ps) == 1 {
		plural = ""
	}
	return errors.New(fmt.Sprintf("%s: %d validation problem%s:\n%s", file, len(ps), plural, strings.Join(lines, "\n")))
}

func validate(c *Config, dir string) []problem {
	var ps []problem
	add := func(line int, path, format string, args ...any) {
		ps = append(ps, problem{line: line, path: path, msg: fmt.Sprintf(format, args...)})
	}

	// site
	if c.Site.Title == "" {
		add(c.Site.Line, "site.title", `missing required field "title"`)
	}
	switch {
	case c.Site.Picture == "":
		add(c.Site.Line, "site.picture", `missing required field "picture"`)
	case isBadFileName(c.Site.Picture):
		add(c.Site.Line, "site.picture", "file %q must be a bare file name (no path separators)", c.Site.Picture)
	default:
		addFileChecks(&ps, c.Site.Picture, c.Site.Line, "site.picture", "picture file", dir)
	}
	if c.Site.Favicon != "" {
		addFileChecks(&ps, c.Site.Favicon, c.Site.Line, "site.favicon", "favicon file", dir)
	}

	// head.meta
	for i := range c.Head.Meta {
		m := &c.Head.Meta[i]
		path := fmt.Sprintf("head.meta[%d]", i)
		set := 0
		for _, v := range []string{m.Name, m.Property, m.HTTPEquiv, m.Charset} {
			if v != "" {
				set++
			}
		}
		if set > 1 {
			add(m.Line, path, "at most one of name, property, http-equiv, charset may be set")
		}
		if m.Content == "" && m.Charset == "" {
			add(m.Line, path, `content is required unless charset is set`)
		}
		if m.Charset != "" && !strings.EqualFold(m.Charset, "utf-8") {
			add(m.Line, path, `charset must be "utf-8"`)
		}
	}

	// head.links
	for i := range c.Head.Links {
		l := &c.Head.Links[i]
		path := fmt.Sprintf("head.links[%d]", i)
		switch {
		case l.Rel == "":
			add(l.Line, path, `missing required field "rel"`)
		case !relAllowed[l.Rel]:
			add(l.Line, path, `invalid rel %q (allowed: stylesheet, preconnect, dns-prefetch, preload, icon, alternate)`, l.Rel)
		}
		if l.Href == "" {
			add(l.Line, path, `missing required field "href"`)
		} else {
			addHrefCheck(add, l.Line, path, l.Href, false)
		}
	}

	// head.scripts
	for i := range c.Head.Scripts {
		s := &c.Head.Scripts[i]
		path := fmt.Sprintf("head.scripts[%d]", i)
		if s.Src == "" {
			add(s.Line, path, `missing required field "src"`)
		} else {
			addHrefCheck(add, s.Line, path, s.Src, false)
		}
	}

	// fonts
	if f := c.Fonts; f != nil {
		if len(f.Families) == 0 {
			add(f.Line, "fonts", "at least one font family is required")
		}
		for i := range f.Families {
			fam := &f.Families[i]
			path := fmt.Sprintf("fonts.families[%d]", i)
			switch {
			case fam.Family == "":
				add(fam.Line, path, `missing required field "family"`)
			case !fontFamilyRe.MatchString(fam.Family):
				add(fam.Line, path, `invalid font family %q (must match ^[A-Za-z0-9 ]+$)`, fam.Family)
			}
			for _, w := range fam.Weights {
				if w < 100 || w > 900 || w%100 != 0 {
					add(fam.Line, path, "font weight %d must be between 100 and 900 in steps of 100", w)
				}
			}
		}
		switch f.Display {
		case "auto", "block", "swap", "fallback", "optional":
		case "":
			add(f.Line, "fonts", "font-display is not set (should default to swap)") // unreachable via Load
		default:
			add(f.Line, "fonts", `invalid font-display %q (allowed: auto, block, swap, fallback, optional)`, f.Display)
		}
		// Google Fonts duplication rule.
		for i := range c.Head.Links {
			l := &c.Head.Links[i]
			if l.Rel == "stylesheet" && strings.Contains(l.Href, "fonts.googleapis.com") {
				add(f.Line, "fonts", "fonts is set and head.links[%d] is a fonts.googleapis.com stylesheet: keep one, remove the other", i)
			}
		}
	}

	// nav links
	if len(c.Links) == 0 {
		add(c.Line, "links", "at least one link entry is required")
	}
	for i := range c.Links {
		l := &c.Links[i]
		path := fmt.Sprintf("links[%d]", i)
		if l.Text == "" {
			add(l.Line, path, `missing required field "text"`)
		}
		if l.Href == "" {
			add(l.Line, path, `missing required field "href"`)
		} else {
			addHrefCheck(add, l.Line, path, l.Href, true) // mailto: allowed in nav links
		}
	}

	// styles: exactly one of styles.scss / styles.css
	_, scssErr := os.Stat(filepath.Join(dir, "styles.scss"))
	_, cssErr := os.Stat(filepath.Join(dir, "styles.css"))
	switch {
	case scssErr == nil && cssErr == nil:
		add(0, "styles", "both styles.scss and styles.css exist; keep exactly one")
	case scssErr != nil && cssErr != nil:
		add(0, "styles", "no styles.scss or styles.css found in config dir")
	}

	return ps
}

var relAllowed = map[string]bool{
	"stylesheet": true, "preconnect": true, "dns-prefetch": true,
	"preload": true, "icon": true, "alternate": true,
}

// addHrefCheck validates one href/src reference: absolute https:// (also
// mailto: when allowMailto) or root-relative (/…); http:// is rejected.
func addHrefCheck(add func(int, string, string, ...any), line int, path, ref string, allowMailto bool) {
	if strings.HasPrefix(ref, "http://") {
		add(line, path, "href %q uses http://; use https:// instead", ref)
		return
	}
	if isAllowedRef(ref, allowMailto) {
		return
	}
	allowed := "an absolute https:// URL or a root-relative path"
	if allowMailto {
		allowed = "an absolute https:// URL, a mailto: address, or a root-relative path"
	}
	add(line, path, "href %q must be %s", ref, allowed)
}

func isAllowedRef(ref string, allowMailto bool) bool {
	if strings.HasPrefix(ref, "/") && !strings.HasPrefix(ref, "//") {
		return true // root-relative
	}
	u, err := url.Parse(ref)
	if err != nil {
		return false
	}
	if u.Scheme == "https" && u.Host != "" {
		return true
	}
	if allowMailto && u.Scheme == "mailto" && u.Opaque != "" {
		return true
	}
	return false
}

// addFileChecks appends problems for a config value that must be a bare,
// non-empty file name and must exist in dir.
func addFileChecks(ps *[]problem, name string, line int, path, kind, dir string) {
	add := func(format string, args ...any) {
		*ps = append(*ps, problem{line: line, path: path, msg: fmt.Sprintf(format, args...)})
	}
	switch {
	case isBadFileName(name):
		add("file %q must be a bare file name (no path separators)", name)
	default:
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			add("%s %q not found in config dir", kind, name)
		}
	}
}

func isBadFileName(name string) bool {
	return name == "." || name == ".." || filepath.Base(name) != name
}
