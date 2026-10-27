package build

import (
	"fmt"
	"io"
	"os"

	"posterpage/internal/config"
)

// File is one emitted file in the build manifest: its base name inside the
// out dir and its size in bytes.
type File struct {
	Name string
	Size int64
}

// Build runs the build pipeline for the resolved config in cfgDir and writes
// the output into outDir (created if needed; previous output is overwritten):
//
//  1. load + validate the config,
//  2. execute the page template → index.html,
//  3. copy site.picture / site.favicon verbatim,
//  4. return the manifest (one File per emitted file).
//
// There is no styles step yet — milestone 3 adds sass compilation / css
// passthrough before the template render, wiring the Input.HasStyles flag.
func Build(cfgDir, outDir string) ([]File, error) {
	cfg, name, err := config.Load(cfgDir)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(cfg, cfgDir, name); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("%s: %w", outDir, err)
	}

	var files []File
	size, err := writeIndex(outDir, cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, File{Name: "index.html", Size: size})

	assetFiles, err := copyAssets(cfgDir, outDir, cfg)
	if err != nil {
		return nil, err
	}
	return append(files, assetFiles...), nil
}

// PrintManifest writes one line per emitted file — name and human-readable
// size — followed by a total line, and nothing else.
func PrintManifest(w io.Writer, files []File) error {
	var total int64
	for _, f := range files {
		if _, err := fmt.Fprintf(w, "%s %s\n", f.Name, humanSize(f.Size)); err != nil {
			return err
		}
		total += f.Size
	}
	_, err := fmt.Fprintf(w, "total %s\n", humanSize(total))
	return err
}

// humanSize renders a byte count as B / kB / MiB (KiB steps of 1024, per-unit
// one decimal above 1000 B).
func humanSize(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"kB", "MiB", "GiB"}
	f := float64(n) / 1024
	for _, u := range units {
		if f < 1024 || u == "GiB" {
			return fmt.Sprintf("%.1f %s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1f GiB", f)
}
