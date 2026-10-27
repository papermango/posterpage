package build

import (
	"bytes"
	"strings"
	"testing"

	"posterpage/internal/config"
)

// fontsURL must produce the css2 URL form: families in listed order with "+"
// for spaces, ":wght@…" only for families that have weights, display always
// appended (defaulting to swap, since fontsURL must stand alone even if it
// somehow never passed through Load).
func TestFontsURL(t *testing.T) {
	t.Run("single family, no weights", func(t *testing.T) {
		got := fontsURL(&config.Fonts{Families: []config.FontFamily{{Family: "Inter"}}})
		if want := "https://fonts.googleapis.com/css2?family=Inter&display=swap"; got != want {
			t.Errorf("fontsURL = %q, want %q", got, want)
		}
	})

	t.Run("one family with weights", func(t *testing.T) {
		got := fontsURL(&config.Fonts{Display: "swap", Families: []config.FontFamily{
			{Family: "Inter", Weights: []int{400, 600}},
		}})
		if want := "https://fonts.googleapis.com/css2?family=Inter:wght@400;600&display=swap"; got != want {
			t.Errorf("fontsURL = %q, want %q", got, want)
		}
	})

	t.Run("multiple families, listed order, escaped space", func(t *testing.T) {
		got := fontsURL(&config.Fonts{Display: "swap", Families: []config.FontFamily{
			{Family: "Inter", Weights: []int{400, 600}},
			{Family: "JetBrains Mono"},
		}})
		want := "https://fonts.googleapis.com/css2?family=Inter:wght@400;600&family=JetBrains+Mono&display=swap"
		if got != want {
			t.Errorf("fontsURL = %q, want %q", got, want)
		}
	})

	t.Run("explicit display passes through", func(t *testing.T) {
		got := fontsURL(&config.Fonts{Display: "optional", Families: []config.FontFamily{{Family: "Inter"}}})
		if want := "https://fonts.googleapis.com/css2?family=Inter&display=optional"; got != want {
			t.Errorf("fontsURL = %q, want %q", got, want)
		}
	})

	t.Run("defensive empty display defaults to swap", func(t *testing.T) {
		// Unreachable via Load (which defaults display), but fontsURL must
		// not emit "display=" (or drop it) on its own.
		got := fontsURL(&config.Fonts{Families: []config.FontFamily{{Family: "Inter", Weights: []int{400}}}})
		if want := "https://fonts.googleapis.com/css2?family=Inter:wght@400&display=swap"; got != want {
			t.Errorf("fontsURL = %q, want %q", got, want)
		}
	})
}

// renderHead renders the Input through the real page template and returns
// just the injected-tags section of <head> — what sits between "<head>\n"
// and "</head>" — so void-vs-script closing and bare attributes are checked
// end-to-end.
func renderHead(t *testing.T, tags []headTag) string {
	t.Helper()
	var buf bytes.Buffer
	in := &Input{Config: &config.Config{}, HeadTags: tags}
	if err := RenderIndex(&buf, in); err != nil {
		t.Fatalf("render: %v", err)
	}
	_, rest, _ := strings.Cut(buf.String(), "</title>\n")
	injected, _, _ := strings.Cut(rest, "</head>")
	return strings.TrimSuffix(injected, "\n")
}

// emptySiteOG is the og/twitter built-in block headTags emits when the site
// section is unstated: title/description/url/picture unset leaves og:type and
// the summary card (og:title/og:description/og:url/og:image are conditional
// on their source being set).
const emptySiteOG = `<meta property="og:type" content="website">` + "\n" +
	`<meta name="twitter:card" content="summary">` + "\n"

func TestHeadTags(t *testing.T) {
	t.Run("fonts block only: two preconnects, then stylesheet", func(t *testing.T) {
		cfg := &config.Config{Fonts: &config.Fonts{Display: "swap",
			Families: []config.FontFamily{{Family: "Inter", Weights: []int{400}}}}}
		got := renderHead(t, headTags(cfg))
		// href values are html/template-escaped URL attribute values: & → &amp;.
		want := emptySiteOG +
			`<link rel="preconnect" href="https://fonts.googleapis.com">` +
			"\n" +
			`<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>` +
			"\n" +
			`<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter:wght@400&amp;display=swap">`
		if got != want {
			t.Errorf("fonts head =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("meta charset only emits one void pair", func(t *testing.T) {
		cfg := &config.Config{Head: config.Head{
			Meta: []config.MetaTag{{Charset: "utf-8"}},
		}}
		got := renderHead(t, headTags(cfg))
		want := emptySiteOG + `<meta charset="utf-8">`
		if got != want {
			t.Errorf("meta head = %q, want %q", got, want)
		}
	})

	t.Run("meta with name and content", func(t *testing.T) {
		cfg := &config.Config{Head: config.Head{
			Meta: []config.MetaTag{{Name: "description.og", Content: "x y z"}},
		}}
		got := renderHead(t, headTags(cfg))
		want := emptySiteOG + `<meta name="description.og" content="x y z">`
		if got != want {
			t.Errorf("meta head = %q, want %q", got, want)
		}
	})

	t.Run("script: defer/async bare, data-* alphabetized", func(t *testing.T) {
		cfg := &config.Config{Head: config.Head{
			Scripts: []config.ScriptTag{
				{
					Src:   "https://example.com/a.js",
					Defer: true,
					Async: true,
					// Map keys are deliberately listed out of alphabetical
					// order; the emit must sort them. data:zeta has an empty
					// value → bare attribute.
					Data: map[string]string{"data-zeta": "", "data-domain": "x.dev", "data-attr": "v"},
				},
			},
		}}
		got := renderHead(t, headTags(cfg))
		// Emit order: src, defer, async, then data-* sorted: data-attr,
		// data-domain, data-zeta (bare). Map iteration order must not matter.
		want := emptySiteOG +
			`<script src="https://example.com/a.js" defer async data-attr="v" data-domain="x.dev" data-zeta></script>`
		if got != want {
			t.Errorf("script head = %q, want %q", got, want)
		}
	})

	t.Run("script: type/integrity/crossorigin/referrerpolicy as pairs", func(t *testing.T) {
		cfg := &config.Config{Head: config.Head{
			Scripts: []config.ScriptTag{
				{
					Src:            "https://example.com/m.js",
					Type:           "module",
					Integrity:      "sha384-abc",
					Crossorigin:    "anonymous",
					Referrerpolicy: "no-referrer",
				},
			},
		}}
		got := renderHead(t, headTags(cfg))
		want := emptySiteOG +
			`<script src="https://example.com/m.js" type="module" integrity="sha384-abc" crossorigin="anonymous" referrerpolicy="no-referrer"></script>`
		if got != want {
			t.Errorf("script head = %q, want %q", got, want)
		}
	})

	t.Run("link entries render rel/href as void tags", func(t *testing.T) {
		cfg := &config.Config{Head: config.Head{
			Links: []config.LinkTag{
				{Rel: "dns-prefetch", Href: "https://cdn.example.com"},
			},
		}}
		got := renderHead(t, headTags(cfg))
		want := emptySiteOG +
			`<link rel="dns-prefetch" href="https://cdn.example.com">`
		if got != want {
			t.Errorf("link head = %q, want %q", got, want)
		}
	})

	t.Run("empty head and no fonts emits only the og/twitter built-ins", func(t *testing.T) {
		got := renderHead(t, headTags(&config.Config{}))
		want := strings.TrimSuffix(emptySiteOG, "\n")
		if got != want {
			t.Errorf("empty head = %q, want only %q", got, want)
		}
	})

	t.Run("order: fonts preconnects before head entries, meta before links before scripts", func(t *testing.T) {
		cfg := &config.Config{
			Head: config.Head{
				Scripts: []config.ScriptTag{{Src: "https://example.com/a.js"}},
				Meta:    []config.MetaTag{{Name: "author", Content: "A"}},
				Links:   []config.LinkTag{{Rel: "alternate", Href: "/a.xml"}},
			},
			Fonts: &config.Fonts{Display: "swap", Families: []config.FontFamily{{Family: "Inter"}}},
		}
		got := renderHead(t, headTags(cfg))
		want := emptySiteOG +
			`<link rel="preconnect" href="https://fonts.googleapis.com">` +
			"\n" +
			`<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>` +
			"\n" +
			`<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter&amp;display=swap">` +
			"\n" +
			`<meta name="author" content="A">` +
			"\n" +
			`<link rel="alternate" href="/a.xml">` +
			"\n" +
			`<script src="https://example.com/a.js"></script>`
		if got != want {
			t.Errorf("ordered head =\n%s\nwant\n%s", got, want)
		}
	})
}

// Auto-escaping runs in the template, never in Go: a value like "a&b" must
// come out HTML-escaped (the template is the only escaping layer).
func TestHeadAutoEscape(t *testing.T) {
	cfg := &config.Config{Head: config.Head{
		Meta: []config.MetaTag{{Name: "author", Content: `a&b "c" <d> 'e'`}},
	}}
	got := renderHead(t, headTags(cfg))
	want := emptySiteOG + `<meta name="author" content="a&amp;b &#34;c&#34; &lt;d&gt; &#39;e&#39;">`
	if got != want {
		t.Errorf("escaped meta = %q, want %q", got, want)
	}
}

// TestOGTags pins the built-in og/twitter basics, in emission order, for the
// full and minimal site sections.
func TestOGTags(t *testing.T) {
	t.Run("full site: url set → absolute og:image joined onto site root", func(t *testing.T) {
		cfg := &config.Config{Site: config.Site{
			Title:       "Parth",
			Description: "Systems engineer",
			Url:         "https://parthdeshmukh.com",
			Picture:     "me.jpg",
		}}
		got := renderHead(t, headTags(cfg))
		want := `<meta property="og:title" content="Parth">` +
			"\n" +
			`<meta property="og:type" content="website">` +
			"\n" +
			`<meta property="og:description" content="Systems engineer">` +
			"\n" +
			`<meta property="og:url" content="https://parthdeshmukh.com">` +
			"\n" +
			`<meta property="og:image" content="https://parthdeshmukh.com/me.jpg">` +
			"\n" +
			`<meta name="twitter:card" content="summary_large_image">`
		if got != want {
			t.Errorf("og head =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("site.root with path: og:image joins under the path", func(t *testing.T) {
		cfg := &config.Config{Site: config.Site{
			Title: "P", Url: "https://example.com/blog/", Picture: "me.jpg",
		}}
		got := renderHead(t, headTags(cfg))
		if !strings.Contains(got, `<meta property="og:image" content="https://example.com/blog/me.jpg">`) {
			t.Errorf("og:image not joined under site path:\n%s", got)
		}
	})

	t.Run("minimal site: no picture → summary card, no og:image/og:url", func(t *testing.T) {
		cfg := &config.Config{Site: config.Site{Title: "Bare"}}
		got := renderHead(t, headTags(cfg))
		want := `<meta property="og:title" content="Bare">` +
			"\n" +
			`<meta property="og:type" content="website">` +
			"\n" +
			`<meta name="twitter:card" content="summary">`
		if got != want {
			t.Errorf("og head =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("url unset keeps og:image relative", func(t *testing.T) {
		cfg := &config.Config{Site: config.Site{Title: "P", Picture: "me.jpg"}}
		got := renderHead(t, headTags(cfg))
		if !strings.Contains(got, `<meta property="og:image" content="me.jpg">`) {
			t.Errorf("og:image not relative:\n%s", got)
		}
	})
}
