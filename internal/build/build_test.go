package build

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"posterpage/internal/config"
)

// Golden-file test: a fixed config (incl. head entries + fonts) must render
// byte-identical HTML (docs/BUILD_PLAN.md, internal/build testing). HasStyles
// is true, as Build sets it, HeadTags is the headTags(cfg) output and NavLinks
// the navLinks(cfg) output, as writeIndex builds them. The golden asserts the
// full emission order: basics → fonts (preconnects + css2 URL) → head entries.
// URL/attribute escaping here is html/template's own (a plain string in a
// quoted-attr-URL context): & → &amp;, + → &#43;, " → &#34;.
func TestRenderIndexGolden(t *testing.T) {
	// Config shape = example/site.yml: link_style: button global default,
	// a mixed nav (buttons plus one style: link entry, itself with a
	// subtitle rendering as <small>). No site.url → og:image stays relative.
	cfg := &config.Config{
		Site: config.Site{
			Title:       "Parth & Co",
			Description: "Systems engineer",
			Bio:         "Systems engineer. I build things.",
			Picture:     "me.jpg",
			Favicon:     "favicon.png",
			LinkStyle:   "button",
		},
		Head: config.Head{
			Meta: []config.MetaTag{
				{Name: "author", Content: "Parth Deshmukh"},
			},
			Links: []config.LinkTag{
				{Rel: "alternate", Href: "/blog.xml"},
			},
			Scripts: []config.ScriptTag{
				{
					Src:   "https://plausible.io/js/script.js",
					Defer: true,
					Data:  map[string]string{"data-domain": "parthdeshmukh.com"},
				},
			},
		},
		Fonts: &config.Fonts{
			Display: "swap",
			Families: []config.FontFamily{
				{Family: "Inter", Weights: []int{400, 600}},
				{Family: "JetBrains Mono"},
			},
		},
		Links: []config.Link{
			{Text: "GitHub", Href: "https://github.com/pdeshmukh"},
			{Text: "Blog", Href: "/blog", Subtitle: "Occasional writing", Style: "link"},
			{Text: "Email me", Href: "mailto:hi@parthdeshmukh.com"},
		},
	}

	var got bytes.Buffer
	if err := RenderIndex(&got, &Input{Config: cfg, HasStyles: true, HeadTags: headTags(cfg), NavLinks: navLinks(cfg)}); err != nil {
		t.Fatalf("render: %v", err)
	}

	// Output uses tabs for body indentation, exactly as the template does.
	want := "" +
		"<!doctype html>\n" +
		"<html lang=\"en\">\n" +
		"<head>\n" +
		"<meta charset=\"utf-8\">\n" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n" +
		"<title>Parth &amp; Co</title>\n" +
		"<meta name=\"description\" content=\"Systems engineer\">\n" +
		"<link rel=\"icon\" href=\"favicon.png\">\n" +
		"<link rel=\"stylesheet\" href=\"styles.css\">\n" +
		// og/twitter basics first among the injected tags (site-derived):
		// picture set and site.url unset → relative og:image, large card.
		"<meta property=\"og:title\" content=\"Parth &amp; Co\">\n" +
		"<meta property=\"og:type\" content=\"website\">\n" +
		"<meta property=\"og:description\" content=\"Systems engineer\">\n" +
		"<meta property=\"og:image\" content=\"me.jpg\">\n" +
		"<meta name=\"twitter:card\" content=\"summary_large_image\">\n" +
		// fonts block, in plan order
		"<link rel=\"preconnect\" href=\"https://fonts.googleapis.com\">\n" +
		"<link rel=\"preconnect\" href=\"https://fonts.gstatic.com\" crossorigin>\n" +
		"<link rel=\"stylesheet\" href=\"https://fonts.googleapis.com/css2?family=Inter:wght@400;600&amp;family=JetBrains&#43;Mono&amp;display=swap\">\n" +
		// then head meta → links → scripts
		"<meta name=\"author\" content=\"Parth Deshmukh\">\n" +
		"<link rel=\"alternate\" href=\"/blog.xml\">\n" +
		"<script src=\"https://plausible.io/js/script.js\" defer data-domain=\"parthdeshmukh.com\"></script>\n" +
		"</head>\n" +
		"<body class=\"bg-body\">\n" +
		"<main class=\"container\" style=\"max-width: 40rem\">\n" +
		"\t<header class=\"text-center\">\n" +
		"\t\t<img class=\"rounded-circle mt-5\" src=\"me.jpg\" width=\"120\" height=\"120\" alt=\"Parth &amp; Co\">\n" +
		"\t\t<h1 class=\"h3 mt-3\">Parth &amp; Co</h1>\n" +
		"\t\t<p class=\"text-body-secondary\">Systems engineer. I build things.</p>\n" +
		"\t</header>\n" +
		"\t<nav class=\"d-grid gap-2 my-4\">\n" +
		"\t\t<a class=\"btn btn-outline-secondary\" href=\"https://github.com/pdeshmukh\">GitHub</a>\n" +
		"\t\t<a class=\"link-secondary link-underline link-underline-opacity-0\" href=\"/blog\">Blog <small class=\"d-block\">Occasional writing</small></a>\n" +
		"\t\t<a class=\"btn btn-outline-secondary\" href=\"mailto:hi@parthdeshmukh.com\">Email me</a>\n" +
		"\t</nav>\n" +
		"</main>\n" +
		"</body>\n" +
		"</html>\n"

	if got.String() != want {
		t.Errorf("rendered HTML mismatch\n--- got ---\n%s\n--- want ---\n%s", got.String(), want)
	}
}

// TestNavLinks pins the render-style resolution: entry style wins over
// site.link_style, absent entries inherit it, and an empty global (configs
// built without Load's defaulting) still renders as the default button.
func TestNavLinks(t *testing.T) {
	tests := []struct {
		name        string
		globalStyle string
		entryStyle  string
		wantButton  bool
	}{
		{"empty global, empty entry", "", "", true},
		{"global link, empty entry", "link", "", false},
		{"global button, entry link", "button", "link", false},
		{"global link, entry button", "link", "button", true},
		{"global link, entry link", "link", "link", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Site:  config.Site{LinkStyle: tc.globalStyle},
				Links: []config.Link{{Text: "Blog", Href: "/blog", Style: tc.entryStyle}},
			}
			got := navLinks(cfg)
			if len(got) != 1 {
				t.Fatalf("navLinks length = %d, want 1", len(got))
			}
			if got[0].IsButton != tc.wantButton {
				t.Errorf("navLinks[0].IsButton = %v, want %v", got[0].IsButton, tc.wantButton)
			}
			want := NavLink{Text: "Blog", Href: "/blog", IsButton: tc.wantButton}
			if got[0] != want {
				t.Errorf("navLinks[0] = %+v, want %+v", got[0], want)
			}
		})
	}
}

func TestCopyAssets(t *testing.T) {
	picBytes := []byte("fake jpeg bytes \x00\x01\x02")
	favBytes := []byte("fake png bytes")

	t.Run("copied verbatim with names preserved", func(t *testing.T) {
		in := t.TempDir()
		out := t.TempDir()
		if err := os.WriteFile(filepath.Join(in, "me.jpg"), picBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(in, "favicon.png"), favBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{Site: config.Site{Picture: "me.jpg", Favicon: "favicon.png"}}

		files, err := copyAssets(in, out, cfg)
		if err != nil {
			t.Fatalf("copyAssets: %v", err)
		}
		if len(files) != 2 || files[0].Name != "me.jpg" || files[1].Name != "favicon.png" {
			t.Fatalf("manifest = %+v, want me.jpg + favicon.png", files)
		}
		gotPic, err := os.ReadFile(filepath.Join(out, "me.jpg"))
		if err != nil {
			t.Fatalf("read copied picture: %v", err)
		}
		gotFav, err := os.ReadFile(filepath.Join(out, "favicon.png"))
		if err != nil {
			t.Fatalf("read copied favicon: %v", err)
		}
		if !bytes.Equal(gotPic, picBytes) || !bytes.Equal(gotFav, favBytes) {
			t.Error("copied asset bytes differ from source")
		}
		if files[0].Size != int64(len(picBytes)) || files[1].Size != int64(len(favBytes)) {
			t.Errorf("manifest sizes = %d/%d, want %d/%d", files[0].Size, files[1].Size, len(picBytes), len(favBytes))
		}
	})

	t.Run("config dir == out dir: skipped, source untouched", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "me.jpg"), picBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{Site: config.Site{Picture: "me.jpg"}}

		files, err := copyAssets(dir, dir, cfg)
		if err != nil {
			t.Fatalf("copyAssets: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("manifest = %+v, want no copied assets for self-copy", files)
		}
		got, err := os.ReadFile(filepath.Join(dir, "me.jpg"))
		if err != nil {
			t.Fatalf("source vanished: %v", err)
		}
		if !bytes.Equal(got, picBytes) {
			t.Error("self-copy corrupted the source file")
		}
	})
}

// Pipeline-level test through the same entry point cmd uses (build.Build +
// build.PrintManifest): an example-like fixture must produce exactly
// index.html + styles.css + the copied assets, with the manifest matching the
// file count. The .css style source doubles as the passthrough path test; the
// styles.scss compile path is covered in styles_test.go.
func TestBuildPipeline(t *testing.T) {
	dir := t.TempDir()
	out := t.TempDir()

	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("site.yml", `site:
  title: Test Site
  description: a test
  bio: Hello there.
  picture: me.jpg
  favicon: favicon.png
links:
  - text: GitHub
    href: https://github.com/pdeshmukh
  - text: Blog
    href: /blog
    subtitle: Occasional writing
`)
	write("styles.css", "/* present so config.Validate passes */\n")
	write("me.jpg", "picture bytes")
	write("favicon.png", "favicon bytes")

	files, err := Build(dir, out)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Out dir contains exactly index.html + styles.css + the two copied
	// assets (styles.css is byte-passthrough of the source).
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if got, want := strings.Join(names, ","), "favicon.png,index.html,me.jpg,styles.css"; got != want {
		t.Errorf("out dir = %q, want %q", got, want)
	}

	if len(files) != 4 {
		t.Fatalf("manifest has %d files, want 4: %+v", len(files), files)
	}
	// Stable manifest order: index.html, styles.css, then picture/favicon.
	wantOrder := []string{"index.html", "styles.css", "me.jpg", "favicon.png"}
	for i, name := range wantOrder {
		if files[i].Name != name || files[i].Size == 0 {
			t.Errorf("manifest[%d] = %+v, want non-empty %s", i, files[i], name)
		}
	}
	srcCss, err := os.ReadFile(filepath.Join(dir, "styles.css"))
	if err != nil {
		t.Fatal(err)
	}
	outCss, err := os.ReadFile(filepath.Join(out, "styles.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(srcCss, outCss) {
		t.Error("styles.css passthrough bytes differ from source")
	}

	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<!doctype html>", `<title>Test Site</title>`,
		`<link rel="stylesheet" href="styles.css">`,
		`<a class="btn btn-outline-secondary" href="https://github.com/pdeshmukh">GitHub</a>`,
	} {
		if !bytes.Contains(html, []byte(want)) {
			t.Errorf("index.html missing %q", want)
		}
	}

	var manifest bytes.Buffer
	if err := PrintManifest(&manifest, files); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(manifest.String(), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("manifest has %d lines, want 5: %q", len(lines), manifest.String())
	}
	if !strings.HasPrefix(lines[len(lines)-1], "total ") {
		t.Errorf("last manifest line = %q, want a total line", lines[len(lines)-1])
	}
}

// TestColorModeRendering pins the site.color_mode emission: the <html> theme
// attribute and — for auto — the SSG-generated inline script as the FIRST
// element inside <head>, ahead of the charset meta (no dark flash). The
// script is a build-time constant, not user config.
func TestColorModeRendering(t *testing.T) {
	render := func(t *testing.T, cfg *config.Config) string {
		t.Helper()
		var buf bytes.Buffer
		in := &Input{Config: cfg, HasStyles: true, HeadTags: headTags(cfg), NavLinks: navLinks(cfg)}
		if cfg.Site.ColorMode == "auto" {
			el := colorModeScriptEl()
			in.ColorScript = &el
		}
		if err := RenderIndex(&buf, in); err != nil {
			t.Fatalf("render: %v", err)
		}
		return buf.String()
	}

	t.Run("light: plain <html>, no script", func(t *testing.T) {
		out := render(t, &config.Config{Site: config.Site{Title: "T", Picture: "me.jpg"}})
		if strings.Contains(out, `data-bs-theme`) || strings.Contains(out, "<script>") {
			t.Errorf("light mode leaked theme attr/script:\n%s", out)
		}
		if !strings.HasPrefix(out, "<!doctype html>\n<html lang=\"en\">") {
			t.Errorf("light <html> tag wrong:\n%s", out)
		}
	})

	t.Run("dark: data-bs-theme=dark, no script", func(t *testing.T) {
		cfg := &config.Config{Site: config.Site{Title: "T", Picture: "me.jpg", ColorMode: "dark"}}
		out := render(t, cfg)
		if !strings.HasPrefix(out, "<!doctype html>\n<html lang=\"en\" data-bs-theme=\"dark\">") {
			t.Errorf("dark <html> tag wrong:\n%s", out)
		}
		if strings.Contains(out, "<script>") {
			t.Errorf("dark mode emitted the auto script:\n%s", out)
		}
	})

	t.Run("auto: data-bs-theme=light + script first in head, before charset", func(t *testing.T) {
		cfg := &config.Config{Site: config.Site{Title: "T", Picture: "me.jpg", ColorMode: "auto"}}
		out := render(t, cfg)
		if !strings.HasPrefix(out, "<!doctype html>\n<html lang=\"en\" data-bs-theme=\"light\">") {
			t.Errorf("auto <html> tag wrong:\n%s", out)
		}
		script := "<script>\n  const dark = window.matchMedia(\"(prefers-color-scheme: dark)\");\n" +
			"  document.documentElement.dataset.bsTheme = dark.matches ? \"dark\" : \"light\";\n</script>"
		if !strings.Contains(out, script) {
			t.Errorf("auto script body =\nwant:\n%s\ngot:\n%s", script, out)
		}
		// Placement: <head>\n<script>…</script> comes before <meta charset>.
		i := strings.Index(out, script)
		if i < 0 {
			t.Fatal("script missing")
		}
		if !strings.HasPrefix(out[i:], script) ||
			!strings.Contains(out[:i], "<head>") ||
			strings.Contains(out[:i], `<meta charset`) {
			t.Errorf("script not before charset;\n%s", out)
		}
	})
}

// TestHumanSize pins the binary-unit labels: B below 1000, then 1024 steps
// marked KiB/MiB/GiB with one decimal.
func TestHumanSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1024, "1.0 KiB"},
		{1500, "1.5 KiB"},
		{29747, "29.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
	}
	for _, tc := range tests {
		if got := humanSize(tc.n); got != tc.want {
			t.Errorf("humanSize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestWriteIndexAutoScript: through writeIndex itself (Build's render step),
// color_mode auto attaches the generated script element — first in <head> —
// and light/dark attach none.
func TestWriteIndexAutoScript(t *testing.T) {
	write := func(colorMode string) []byte {
		t.Helper()
		out := t.TempDir()
		cfg := &config.Config{Site: config.Site{Title: "T", Picture: "me.jpg", ColorMode: colorMode}}
		if _, err := writeIndex(out, cfg); err != nil {
			t.Fatalf("writeIndex(%s): %v", colorMode, err)
		}
		html, err := os.ReadFile(filepath.Join(out, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		return html
	}
	cutHead := func(html string) string {
		_, chunk, ok := strings.Cut(html, "<head>\n")
		if !ok {
			t.Fatalf("<head> missing:\n%s", html)
		}
		return chunk
	}
	auto, light, dark := write("auto"), write("light"), write("dark")
	if !strings.HasPrefix(cutHead(string(auto)), "<script>") {
		t.Errorf("auto mode: script not first in <head>:\n%s", auto)
	}
	if i, attrIdx := strings.Index(cutHead(string(light)), "<script>"), strings.Index(string(light), "data-bs-theme"); i >= 0 || attrIdx >= 0 {
		t.Errorf("light mode: unexpected script/theme attr:\n%s", light)
	}
	if !strings.Contains(string(dark), `<html lang="en" data-bs-theme="dark">`) || strings.Contains(cutHead(string(dark)), "<script>") {
		t.Errorf("dark mode html tag/script wrong:\n%s", dark)
	}
}
