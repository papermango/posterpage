package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Styles-step tests run with the sass binary when it is present; skipped
// gracefully otherwise (docs/BUILD_PLAN.md, internal/build testing).
func requireSass(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sass"); err != nil {
		t.Skip("sass binary not found; skipping styles compile tests")
	}
}

// TestBuildStylesPassthrough: a styles.css config source is copied to the out
// dir byte for byte, with a manifest entry.
func TestBuildStylesPassthrough(t *testing.T) {
	src := []byte("/* plain css, untouched */\np { color: red }\n")
	cfg, out := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(out, 0o755); err != nil { // keep buildStyles simple: out dir exists
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "styles.css"), src, 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := buildStyles(cfg, out)
	if err != nil {
		t.Fatalf("buildStyles: %v", err)
	}
	if f.Name != "styles.css" || f.Size != int64(len(src)) {
		t.Errorf("manifest entry = %+v, want styles.css size %d", f, len(src))
	}
	got, err := os.ReadFile(filepath.Join(out, "styles.css"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(src) {
		t.Errorf("passthrough not byte-identical: got %q, want %q", got, src)
	}
}

// TestBuildStylesScssCompile: a small self-contained styles.scss (no
// bootstrap import) is compiled — minified output exists, manifest entry set.
func TestBuildStylesScssCompile(t *testing.T) {
	requireSass(t)

	cfg, out := t.TempDir(), t.TempDir()
	scss := "body {\n  margin: 0 auto;\n  color: #123;\n}\n"
	if err := os.WriteFile(filepath.Join(cfg, "styles.scss"), []byte(scss), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := buildStyles(cfg, out)
	if err != nil {
		t.Fatalf("buildStyles: %v", err)
	}
	if f.Name != "styles.css" || f.Size == 0 {
		t.Fatalf("manifest entry = %+v, want non-zero styles.css", f)
	}
	got, err := os.ReadFile(filepath.Join(out, "styles.css"))
	if err != nil {
		t.Fatal(err)
	}
	// --style=compressed output of the fixture.
	if want := "body{margin:0 auto;color:#123}"; strings.TrimSpace(string(got)) != want {
		t.Errorf("compiled css = %q, want %q", strings.TrimSpace(string(got)), want)
	}
}

// TestBuildStylesBootstrapVendored: a tiny file importing the vendored
// Bootstrap variables must compile against the load path — this exercises the
// embedded scss being extracted per build run and handed to sass, without a
// full bootstrap build (the full example compile is a manual gate, not CI).
// functions + mixins precede variables, as bootstrap.scss itself does:
// _variables.scss calls mixins defined in _mixins.scss.
func TestBuildStylesBootstrapVendored(t *testing.T) {
	requireSass(t)

	cfg, out := t.TempDir(), t.TempDir()
	scss := "@import \"functions\";\n@import \"mixins\";\n@import \"variables\";\nbody { color: $primary }\n"
	if err := os.WriteFile(filepath.Join(cfg, "styles.scss"), []byte(scss), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := buildStyles(cfg, out); err != nil {
		t.Fatalf("buildStyles with vendored bootstrap import: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(out, "styles.css"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "body{color:#0d6efd}"; !strings.Contains(string(got), want) {
		t.Errorf("compiled css missing %q: got %q", want, strings.TrimSpace(string(got)))
	}
}

// TestBuildStylesSassMissing: when sass is not on PATH the error is exactly
// the friendly install hint.
func TestBuildStylesSassMissing(t *testing.T) {
	if _, err := exec.LookPath("sass"); err != nil {
		t.Skip("sass already absent; nothing to test")
	}
	t.Setenv("PATH", "/nonexistent")

	cfg, out := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg, "styles.scss"), []byte("p { color: red }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := buildStyles(cfg, out)
	if err == nil {
		t.Fatal("buildStyles unexpectedly succeeded without sass")
	}
	if want := "sass not found — install with: brew install sass/sass/sass"; err.Error() != want {
		t.Errorf("error = %q, want exactly %q", err.Error(), want)
	}
}

// TestBuildSassFailureAbortsBeforeIndex: through the Build entry point, a
// styles.scss with a syntax error surfaces a sass: <stderr tail> error and —
// because the styles step runs before the render — index.html is not written.
func TestBuildSassFailureAbortsBeforeIndex(t *testing.T) {
	requireSass(t)

	dir, out := t.TempDir(), t.TempDir()
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
links:
  - text: GitHub
    href: https://github.com/pdeshmukh
`)
	write("me.jpg", "picture bytes")
	write("styles.scss", "p {\n  color: ;\n") // broken: bad value, unclosed brace

	_, err := Build(dir, out)
	if err == nil {
		t.Fatal("Build unexpectedly succeeded with broken styles.scss")
	}
	if !strings.Contains(err.Error(), "sass:") {
		t.Errorf("error = %q, want a sass: <stderr tail> wrapper", err)
	}
	if _, statErr := os.Stat(filepath.Join(out, "index.html")); !os.IsNotExist(statErr) {
		t.Errorf("index.html written despite failed build (stat err: %v)", statErr)
	}
}
