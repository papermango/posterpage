package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"posterpage/internal/config"
)

// copyAssets copies site.picture and site.favicon (if set) verbatim from the
// config dir to the out dir, preserving their names (config validation
// guarantees bare file names and existence; no further checks here). It
// returns one File per asset it actually copied — assets are skipped (and
// left out of the manifest) when source and destination are the same file,
// i.e. the config dir and the out dir resolve to the same directory.
func copyAssets(cfgDir, outDir string, cfg *config.Config) ([]File, error) {
	var files []File
	for _, name := range []string{cfg.Site.Picture, cfg.Site.Favicon} {
		if name == "" {
			continue
		}
		size, copied, err := copyAsset(cfgDir, outDir, name)
		if err != nil {
			return nil, err
		}
		if copied {
			files = append(files, File{Name: name, Size: size})
		}
	}
	return files, nil
}

// copyAsset byte-for-byte copies name from cfgDir to outDir.
func copyAsset(cfgDir, outDir, name string) (size int64, copied bool, err error) {
	src, dst := filepath.Join(cfgDir, name), filepath.Join(outDir, name)
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return 0, false, err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return 0, false, err
	}
	if srcAbs == dstAbs {
		return 0, false, nil // self-copy guard: cfg dir == out dir
	}

	in, err := os.Open(src)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", name, err)
	}
	defer in.Close()

	st, err := in.Stat()
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", name, err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", name, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return 0, false, fmt.Errorf("%s: %w", name, err)
	}
	if err := out.Close(); err != nil {
		return 0, false, fmt.Errorf("%s: %w", name, err)
	}
	return st.Size(), true, nil
}
