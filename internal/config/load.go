package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// configNames is the fixed candidate order; "found" reports preserve it.
var configNames = []string{"site.yml", "site.yaml", "site.json"}

// Load finds exactly one config file in dir (zero or multiple is an error),
// decodes it into a Config, applies defaults, and returns the config together
// with the config file's base name.
func Load(dir string) (*Config, string, error) {
	var found []string
	for _, name := range configNames {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			found = append(found, name)
		}
	}
	switch len(found) {
	case 0:
		return nil, "", fmt.Errorf("no config file found in %s (want exactly one of %s)", dir, strings.Join(configNames, ", "))
	case 1:
	default:
		return nil, "", fmt.Errorf("multiple config files found in %s: %s (want exactly one)", dir, strings.Join(found, ", "))
	}
	name := found[0]

	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()

	var c Config
	if filepath.Ext(name) == ".json" {
		dec := json.NewDecoder(f)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			return nil, "", fmt.Errorf("%s: %w", name, err)
		}
	} else {
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		if err := dec.Decode(&c); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "", fmt.Errorf("%s: config file is empty", name)
			}
			var te *yaml.TypeError
			if errors.As(err, &te) {
				if len(te.Errors) > 1 {
					return nil, "", fmt.Errorf("%s: yaml: unmarshal errors:\n  %s", name, strings.Join(te.Errors, "\n  "))
				}
				return nil, "", fmt.Errorf("%s: %s", name, te.Errors[0])
			}
			return nil, "", fmt.Errorf("%s: %s", name, strings.TrimPrefix(err.Error(), "yaml: "))
		}
	}

	applyDefaults(&c)
	return &c, name, nil
}

// applyDefaults resolves the documented defaults on the loaded config: the
// font-display, the global link render style, and the color mode. Per-entry
// links[].style is an override and stays empty when absent.
func applyDefaults(c *Config) {
	if c.Fonts != nil && c.Fonts.Display == "" {
		c.Fonts.Display = "swap"
	}
	if c.Site.LinkStyle == "" {
		c.Site.LinkStyle = "button"
	}
	if c.Site.ColorMode == "" {
		c.Site.ColorMode = "light"
	}
}
