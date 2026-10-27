package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFiles creates a fixture config dir in t.TempDir() and returns its path.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if d := filepath.Dir(p); d != dir {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runCase loads and validates a fixture dir, returning the first error from
// either stage (nil on success).
func runCase(t *testing.T, files map[string]string) (*Config, error) {
	t.Helper()
	dir := writeFiles(t, files)
	cfg, name, err := Load(dir)
	if err != nil {
		return nil, err
	}
	return cfg, Validate(cfg, dir, name)
}

func validYAML() string {
	return `site:
  title: Parth Deshmukh
  description: Systems engineer
  bio: |
    Systems engineer. I build things.
  picture: me.jpg
  favicon: favicon.png
  link_style: button
head:
  meta:
    - name: author
      content: Parth Deshmukh
  links:
    - rel: preconnect
      href: https://fonts.googleapis.com
  scripts:
    - src: https://plausible.io/js/script.js
      defer: true
      data-domain: parthdeshmukh.com
fonts:
  display: swap
  families:
    - family: Inter
      weights: [400, 600]
    - family: JetBrains Mono
links:
  - text: GitHub
    href: https://github.com/parthdeshmukh
  - text: Blog
    href: /blog
    subtitle: Occasional writing
    style: link
  - text: Email me
    href: mailto:hi@example.com
`
}

func validJSON() string {
	return `{
  "site": {
    "title": "Parth Deshmukh",
    "picture": "me.jpg",
    "favicon": "favicon.png"
  },
  "fonts": {
    "families": [{"family": "Inter", "weights": [400, 600]}]
  },
  "links": [
    {"text": "GitHub", "href": "https://github.com/parthdeshmukh"}
  ]
}`
}

// base is the set of non-config files a working config dir needs.
func base(extra map[string]string) map[string]string {
	files := map[string]string{
		"styles.scss": "", // any bytes: milestone 1 only checks existence
		"me.jpg":      "jpeg",
		"favicon.png": "png",
	}
	for k, v := range extra {
		files[k] = v
	}
	return files
}

func TestLoadAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string // substring; empty = expect success
		check   func(t *testing.T, c *Config)
	}{
		{
			name:  "valid yaml",
			files: base(map[string]string{"site.yml": validYAML()}),
			check: func(t *testing.T, c *Config) {
				if c.Site.Title != "Parth Deshmukh" || c.Site.Picture != "me.jpg" {
					t.Errorf("site not decoded: %+v", c.Site)
				}
				if c.Fonts == nil || c.Fonts.Display != "swap" {
					t.Errorf("fonts.display default not applied: %+v", c.Fonts)
				}
				if len(c.Links) != 3 {
					t.Errorf("links length = %d, want 3", len(c.Links))
				}
				if c.Links[1].Style != "link" {
					t.Errorf("links[1].style = %q, want %q", c.Links[1].Style, "link")
				}
				if len(c.Head.Scripts) != 1 || c.Head.Scripts[0].Data["data-domain"] != "parthdeshmukh.com" {
					t.Errorf("script data attrs not decoded: %+v", c.Head.Scripts)
				}
				if c.Site.Line == 0 {
					t.Error("site.Line not attached")
				}
			},
		},
		{
			name:  "valid json",
			files: base(map[string]string{"site.json": validJSON()}),
			check: func(t *testing.T, c *Config) {
				if c.Site.Title != "Parth Deshmukh" {
					t.Errorf("Title = %q", c.Site.Title)
				}
				if c.Fonts == nil || c.Fonts.Display != "swap" {
					t.Errorf("fonts.display default not applied: %+v", c.Fonts)
				}
			},
		},
		{
			name:    "no config file",
			files:   base(nil),
			wantErr: "no config file found in",
		},
		{
			name:    "two config files",
			files:   base(map[string]string{"site.yml": validYAML(), "site.yaml": validYAML()}),
			wantErr: "multiple config files found",
		},
		{
			name: "missing site.picture",
			files: base(map[string]string{"site.yml": `site:
  title: T
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: `site.picture: missing required field "picture"`,
		},
		{
			name: "missing picture file",
			files: map[string]string{
				"site.yml": `site:
  title: T
  picture: me.jpg
links:
  - text: GitHub
    href: https://example.com/x
`,
				"styles.scss": "",
				"favicon.png": "png",
			},
			wantErr: `picture file "me.jpg" not found in config dir`,
		},
		{
			name: "unknown field in yaml",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
links:
  - text: GitHub
    href: https://example.com/x
footers: []
`}),
			wantErr: "site.yml: line 7: field footers not found in type config.Config",
		},
		{
			name: "script missing src",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
head:
  scripts:
    - defer: true
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: `head.scripts[0]: missing required field "src"`,
		},
		{
			name: "script with valid data-domain",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
head:
  scripts:
    - src: https://plausible.io/js/script.js
      data-domain: example.com
links:
  - text: GitHub
    href: https://example.com/x
`}),
			check: func(t *testing.T, c *Config) {
				if c.Head.Scripts[0].Data["data-domain"] != "example.com" {
					t.Errorf("Data = %+v", c.Head.Scripts[0].Data)
				}
			},
		},
		{
			name: "script with non-data unknown attr",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
head:
  scripts:
    - src: https://plausible.io/js/script.js
      onclick: "evil()"
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: `field onclick not found in type config.ScriptTag`,
		},
		{
			name: "bad rel",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
head:
  links:
    - rel: prefetch
      href: https://example.com/style.css
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: `invalid rel "prefetch"`,
		},
		{
			name: "bad font weight",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
fonts:
  families:
    - family: Inter
      weights: [350]
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: "font weight 350 must be between 100 and 900 in steps of 100",
		},
		{
			name: "invalid site.link_style",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
  link_style: pill
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: `site.link_style: invalid link style "pill" (allowed: button, link)`,
		},
		{
			name: "invalid per-entry style",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
links:
  - text: GitHub
    href: https://example.com/x
  - text: Blog
    href: /blog
    style: banner
`}),
			wantErr: `links[1]: invalid style "banner" (allowed: button, link)`,
		},
		{
			name: "link styles absent → defaults applied",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
links:
  - text: GitHub
    href: https://example.com/x
`}),
			check: func(t *testing.T, c *Config) {
				if c.Site.LinkStyle != "button" {
					t.Errorf("site.link_style = %q, want defaulted %q", c.Site.LinkStyle, "button")
				}
				if c.Links[0].Style != "" {
					t.Errorf("links[0].style = %q, want %q (inherit)", c.Links[0].Style, "")
				}
			},
		},
		{
			name: "per-link override accepted",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
  link_style: link
links:
  - text: GitHub
    href: https://example.com/x
    style: button
  - text: Blog
    href: /blog
    subtitle: Occasional writing
    style: link
`}),
			check: func(t *testing.T, c *Config) {
				if c.Site.LinkStyle != "link" || c.Links[0].Style != "button" || c.Links[1].Style != "link" {
					t.Errorf("link styles not decoded: site %q, links %q/%q",
						c.Site.LinkStyle, c.Links[0].Style, c.Links[1].Style)
				}
			},
		},
		{
			name: "http href rejected",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
links:
  - text: Blog
    href: http://example.com/blog
`}),
			wantErr: `href "http://example.com/blog" uses http://; use https:// instead`,
		},
		{
			name: "duplicate fonts rule",
			files: base(map[string]string{"site.yml": `site:
  title: T
  picture: me.jpg
head:
  links:
    - rel: stylesheet
      href: https://fonts.googleapis.com/css2?family=Inter
fonts:
  families:
    - family: Inter
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: "fonts is set and head.links[0] is a fonts.googleapis.com stylesheet: keep one, remove the other",
		},
		{
			name: "both styles files",
			files: base(map[string]string{
				"site.yml": `site:
  title: T
  picture: me.jpg
links:
  - text: GitHub
    href: https://example.com/x
`,
				"styles.css": "",
			}),
			wantErr: "both styles.scss and styles.css exist; keep exactly one",
		},
		{
			name: "neither styles file",
			files: map[string]string{
				"site.yml": `site:
  title: T
  picture: me.jpg
links:
  - text: GitHub
    href: https://example.com/x
`,
				"me.jpg": "jpeg",
			},
			wantErr: "no styles.scss or styles.css found in config dir",
		},
		{
			name: "json unknown field",
			files: base(map[string]string{"site.json": `{
  "site": {"title": "T", "picture": "me.jpg"},
  "footers": [],
  "links": [{"text": "GitHub", "href": "https://example.com/x"}]
}`}),
			wantErr: `unknown field "footers"`,
		},
		{
			name: "multiple problems in one error",
			files: base(map[string]string{"site.yml": `site:
  bio: just a bio
links:
  - text: GitHub
    href: https://example.com/x
`}),
			wantErr: "2 validation problems:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := runCase(t, tc.files)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got success", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

// TestLoadErrorFormat pins the file-prefixed decode error format.
func TestLoadErrorFormat(t *testing.T) {
	dir := writeFiles(t, base(map[string]string{"site.yml": "site: [1, 2]\n"}))
	_, _, err := Load(dir)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if got := err.Error(); !strings.HasPrefix(got, "site.yml: line 1:") {
		t.Errorf("error %q not prefixed with \"site.yml: line 1:\"", got)
	}
}

// TestPrintJSONExcludesLines exercises the build command's config printout.
func TestPrintJSONExcludesLines(t *testing.T) {
	cfg, err := runCase(t, base(map[string]string{"site.yml": validYAML()}))
	if err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}
	var b strings.Builder
	if err := PrintJSON(&b, cfg); err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}
	out := b.String()
	if strings.Contains(out, `"Line"`) {
		t.Error("printout leaks line fields")
	}
	for _, want := range []string{
		`"title": "Parth Deshmukh"`,
		`"data-domain": "parthdeshmukh.com"`,
		`"display": "swap"`,
		`"family": "JetBrains Mono"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("printout missing %s:\n%s", want, out)
		}
	}
}
