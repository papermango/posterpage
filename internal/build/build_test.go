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

// Golden-file test: a fixed minimal config must render byte-identical HTML
// (docs/BUILD_PLAN.md, internal/build testing). HasStyles is true, as Build
// sets it after the styles step.
func TestRenderIndexGolden(t *testing.T) {
	cfg := &config.Config{
		Site: config.Site{
			Title:       "Parth & Co",
			Description: "Systems engineer",
			Bio:         "Systems engineer. I build things.",
			Picture:     "me.jpg",
			Favicon:     "favicon.png",
		},
		Links: []config.Link{
			{Text: "GitHub", Href: "https://github.com/pdeshmukh"},
			{Text: "Blog", Href: "/blog", Subtitle: "Occasional writing"},
		},
	}

	var got bytes.Buffer
	if err := RenderIndex(&got, &Input{Config: cfg, HasStyles: true}); err != nil {
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
		"</head>\n" +
		"<body class=\"bg-body\">\n" +
		"<main class=\"container\" style=\"max-width: 40rem\">\n" +
		"\t<img class=\"rounded-circle mt-5\" src=\"me.jpg\" width=\"120\" height=\"120\" alt=\"Parth &amp; Co\">\n" +
		"\t<h1 class=\"h3 mt-3\">Parth &amp; Co</h1>\n" +
		"\t<p class=\"text-body-secondary\">Systems engineer. I build things.</p>\n" +
		"\t<nav class=\"d-grid gap-2 my-4\">\n" +
		"\t\t<a class=\"btn btn-outline-secondary\" href=\"https://github.com/pdeshmukh\">GitHub</a>\n" +
		"\t\t<a class=\"btn btn-outline-secondary\" href=\"/blog\">Blog <small class=\"d-block\">Occasional writing</small></a>\n" +
		"\t</nav>\n" +
		"</main>\n" +
		"</body>\n" +
		"</html>\n"

	if got.String() != want {
		t.Errorf("rendered HTML mismatch\n--- got ---\n%s\n--- want ---\n%s", got.String(), want)
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
