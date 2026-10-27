package build

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"posterpage/assets"
)

// The two style sources allowed in the config dir (exactly one, checked by
// config.Validate).
const (
	styleScss = "styles.scss"
	styleCss  = "styles.css"
)

// maxSassErrLines caps how many tail lines of dart-sass stderr a wrapped
// error shows — its diagnostics are verbose and the fatal error is at the end.
const maxSassErrLines = 20

// buildStyles produces <outDir>/styles.css and returns its manifest entry.
// styles.css in the config dir is copied through byte for byte; styles.scss
// is compiled by the sass binary (dart-sass) with the vendored Bootstrap scss
// on the load path. Config validation guarantees exactly one source exists;
// the defensive default here mirrors the validation message.
func buildStyles(cfgDir, outDir string) (File, error) {
	scssPath, cssPath := filepath.Join(cfgDir, styleScss), filepath.Join(cfgDir, styleCss)
	_, scssErr := os.Stat(scssPath)
	_, cssErr := os.Stat(cssPath)
	switch {
	case scssErr == nil:
		return compileSass(scssPath, outDir)
	case cssErr == nil:
		return passthroughCss(cssPath, cfgDir, outDir)
	default:
		return File{}, fmt.Errorf("no %s or %s found in config dir", styleScss, styleCss)
	}
}

// passthroughCss copies a styles.css source verbatim to <outDir>/styles.css
// (copyAsset's same-name path), keeping the manifest entry even when cfg dir
// == out dir — unlike picture/favicon, styles.css is always part of the
// generated site and referenced from index.html.
func passthroughCss(src, cfgDir, outDir string) (File, error) {
	size, copied, err := copyAsset(cfgDir, outDir, styleCss)
	if err != nil {
		return File{}, err
	}
	if !copied { // self-copy guard hit: cfg dir == out dir
		st, err := os.Stat(src)
		if err != nil {
			return File{}, err
		}
		size = st.Size()
	}
	return File{Name: styleCss, Size: size}, nil
}

// compileSass runs the pipeline invocation on src (a styles.scss in the
// config dir, absolute or relative to the caller's cwd) and writes compressed
// CSS to <outDir>/styles.css:
//
//	sass --quiet --style=compressed --no-source-map \
//	  --load-path=<vendored scss dir> <src> <outDir>/styles.css
//
// --quiet instead of the more surgical --silence-deprecation lists: dart-sass
// 1.106 flags several deprecations in Bootstrap 5.3.8's scss (import,
// global-builtin, if-function, color-functions — legacy @import-era syntax
// that Bootstrap drops only on its move to @use), and a fixed list goes stale
// with every sass release. --quiet suppresses warnings only; compile errors
// still hit stderr and are surfaced through the nonzero exit path. All paths
// passed to sass are absolute and stderr is captured, so cwd does not matter.
func compileSass(src, outDir string) (File, error) {
	if _, err := exec.LookPath("sass"); err != nil {
		return File{}, errors.New("sass not found — install with: brew install sass/sass/sass")
	}

	// The vendored Bootstrap sources are embedded in the binary; extract them
	// to a fresh temp dir per build run and hand that to sass as load path,
	// so an installed posterpage binary works without the source checkout.
	tmp, err := os.MkdirTemp("", "posterpage-bootstrap-")
	if err != nil {
		return File{}, fmt.Errorf("extract vendored bootstrap scss: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := assets.Extract(tmp); err != nil {
		return File{}, fmt.Errorf("extract vendored bootstrap scss: %w", err)
	}

	src, err = filepath.Abs(src)
	if err != nil {
		return File{}, err
	}
	out := filepath.Join(outDir, styleCss)
	out, err = filepath.Abs(out)
	if err != nil {
		return File{}, err
	}

	var stderr bytes.Buffer
	cmd := exec.Command("sass",
		"--quiet", "--style=compressed", "--no-source-map",
		"--load-path="+filepath.Join(tmp, "bootstrap"),
		src, out,
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return File{}, fmt.Errorf("sass: %s", stderrTail(stderr.String(), maxSassErrLines))
	}

	st, err := os.Stat(out)
	if err != nil {
		return File{}, err
	}
	return File{Name: styleCss, Size: st.Size()}, nil
}

// stderrTail returns the last n lines of s (trailing blank line trimmed) —
// for dart-sass diagnostics the tail carries the fatal error.
func stderrTail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
