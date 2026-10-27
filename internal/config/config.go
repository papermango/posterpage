// Package config defines the posterpage config schema, loads site.yml /
// site.yaml / site.json into it, and validates it against the allowlists in
// docs/BUILD_PLAN.md.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Every config type has a Line field holding the source line of its YAML
// mapping node, attached via UnmarshalYAML. Line fields are excluded from
// JSON output ("-" tags); JSON configs carry no line numbers.

// Config is the root of site.yml / site.json.
type Config struct {
	Line  int    `yaml:"-" json:"-"`
	Site  Site   `yaml:"site" json:"site"`
	Head  Head   `yaml:"head,omitempty" json:"head,omitzero"`
	Fonts *Fonts `yaml:"fonts,omitempty" json:"fonts,omitempty"`
	Links []Link `yaml:"links" json:"links"`
}

type Site struct {
	Line        int    `yaml:"-" json:"-"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Bio         string `yaml:"bio,omitempty" json:"bio,omitempty"`
	Picture     string `yaml:"picture" json:"picture"`
	Favicon     string `yaml:"favicon,omitempty" json:"favicon,omitempty"`
	LinkStyle   string `yaml:"link_style,omitempty" json:"link_style,omitempty"`
}

type Head struct {
	Line    int         `yaml:"-" json:"-"`
	Meta    []MetaTag   `yaml:"meta,omitempty" json:"meta,omitempty"`
	Links   []LinkTag   `yaml:"links,omitempty" json:"links,omitempty"`
	Scripts []ScriptTag `yaml:"scripts,omitempty" json:"scripts,omitempty"`
}

type MetaTag struct {
	Line      int    `yaml:"-" json:"-"`
	Name      string `yaml:"name,omitempty" json:"name,omitempty"`
	Property  string `yaml:"property,omitempty" json:"property,omitempty"`
	HTTPEquiv string `yaml:"http-equiv,omitempty" json:"http-equiv,omitempty"`
	Content   string `yaml:"content,omitempty" json:"content,omitempty"`
	Charset   string `yaml:"charset,omitempty" json:"charset,omitempty"`
}

type LinkTag struct {
	Line int    `yaml:"-" json:"-"`
	Rel  string `yaml:"rel" json:"rel"`
	Href string `yaml:"href" json:"href"`
}

// ScriptTag represents a <script src=…> tag. Inline script bodies are not
// representable at all: there is no body field, and any key outside the
// fixed attribute set must be a data-* attribute (decoded into Data via a
// yaml inline map).
type ScriptTag struct {
	Line           int    `yaml:"-" json:"-"`
	Src            string `yaml:"src" json:"src"`
	Type           string `yaml:"type,omitempty" json:"type,omitempty"`
	Defer          bool   `yaml:"defer,omitempty" json:"defer,omitempty"`
	Async          bool   `yaml:"async,omitempty" json:"async,omitempty"`
	Integrity      string `yaml:"integrity,omitempty" json:"integrity,omitempty"`
	Crossorigin    string `yaml:"crossorigin,omitempty" json:"crossorigin,omitempty"`
	Referrerpolicy string `yaml:"referrerpolicy,omitempty" json:"referrerpolicy,omitempty"`

	// data-* attributes (yaml inline map); marshaled inline for JSON too.
	Data map[string]string `yaml:",inline" json:"-"`
}

type Fonts struct {
	Line     int          `yaml:"-" json:"-"`
	Display  string       `yaml:"display,omitempty" json:"display,omitempty"`
	Families []FontFamily `yaml:"families" json:"families"`
}

type FontFamily struct {
	Line    int    `yaml:"-" json:"-"`
	Family  string `yaml:"family" json:"family"`
	Weights []int  `yaml:"weights,omitempty" json:"weights,omitempty"`
}

type Link struct {
	Line     int    `yaml:"-" json:"-"`
	Text     string `yaml:"text" json:"text"`
	Href     string `yaml:"href" json:"href"`
	Subtitle string `yaml:"subtitle,omitempty" json:"subtitle,omitempty"`
	// Style overrides site.link_style for this entry; empty means "inherit
	// the site default" (per-link, unlike link_style itself, is never
	// defaulted).
	Style string `yaml:"style,omitempty" json:"style,omitempty"`
}

var (
	dataAttrRe  = regexp.MustCompile(`^data-[a-z0-9_-]+$`)
	fontFamilyRe = regexp.MustCompile(`^[A-Za-z0-9 ]+$`)
)

// keySet returns the yaml key -> present set for a struct type, from yaml
// tags: "-" and ",inline" fields are excluded. Used by the UnmarshalYAML
// implementations to reject unknown keys (yaml.Node.Decode does not carry
// KnownFields into nested Decode calls, so the check is done here).
func keySet[T any]() map[string]bool {
	t := reflect.TypeOf(*new(T))
	m := make(map[string]bool, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "-" || strings.Contains(opts, "inline") {
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		m[name] = true
	}
	return m
}

// checkUnknown returns a yaml.TypeError naming every mapping key on n that is
// not allowed, in the same format the decoder itself produces.
func checkUnknown(n *yaml.Node, typ string, allowed map[string]bool, extra func(string) bool) error {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	var errs []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if allowed[key] || (extra != nil && extra(key)) {
			continue
		}
		errs = append(errs, fmt.Sprintf("line %d: field %s not found in type %s", n.Content[i].Line, key, typ))
	}
	if len(errs) == 0 {
		return nil
	}
	return &yaml.TypeError{Errors: errs}
}

func (c *Config) UnmarshalYAML(n *yaml.Node) error {
	type raw Config
	if err := n.Decode((*raw)(c)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.Config", keySet[Config](), nil); err != nil {
		return err
	}
	c.Line = n.Line
	return nil
}

func (s *Site) UnmarshalYAML(n *yaml.Node) error {
	type raw Site
	if err := n.Decode((*raw)(s)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.Site", keySet[Site](), nil); err != nil {
		return err
	}
	s.Line = n.Line
	return nil
}

func (h *Head) UnmarshalYAML(n *yaml.Node) error {
	type raw Head
	if err := n.Decode((*raw)(h)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.Head", keySet[Head](), nil); err != nil {
		return err
	}
	h.Line = n.Line
	return nil
}

func (m *MetaTag) UnmarshalYAML(n *yaml.Node) error {
	type raw MetaTag
	if err := n.Decode((*raw)(m)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.MetaTag", keySet[MetaTag](), nil); err != nil {
		return err
	}
	m.Line = n.Line
	return nil
}

func (l *LinkTag) UnmarshalYAML(n *yaml.Node) error {
	type raw LinkTag
	if err := n.Decode((*raw)(l)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.LinkTag", keySet[LinkTag](), nil); err != nil {
		return err
	}
	l.Line = n.Line
	return nil
}

func (s *ScriptTag) UnmarshalYAML(n *yaml.Node) error {
	type raw ScriptTag
	if err := n.Decode((*raw)(s)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.ScriptTag", keySet[ScriptTag](), func(k string) bool {
		return dataAttrRe.MatchString(k)
	}); err != nil {
		return err
	}
	s.Line = n.Line
	return nil
}

func (f *Fonts) UnmarshalYAML(n *yaml.Node) error {
	type raw Fonts
	if err := n.Decode((*raw)(f)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.Fonts", keySet[Fonts](), nil); err != nil {
		return err
	}
	f.Line = n.Line
	return nil
}

func (f *FontFamily) UnmarshalYAML(n *yaml.Node) error {
	type raw FontFamily
	if err := n.Decode((*raw)(f)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.FontFamily", keySet[FontFamily](), nil); err != nil {
		return err
	}
	f.Line = n.Line
	return nil
}

func (l *Link) UnmarshalYAML(n *yaml.Node) error {
	type raw Link
	if err := n.Decode((*raw)(l)); err != nil {
		return err
	}
	if err := checkUnknown(n, "config.Link", keySet[Link](), nil); err != nil {
		return err
	}
	l.Line = n.Line
	return nil
}

// MarshalJSON folds the yaml-inline Data map back into the JSON object so the
// config printout round-trips data-* attributes.
func (s ScriptTag) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(s.Data)+7)
	if s.Src != "" {
		m["src"] = s.Src
	}
	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Defer {
		m["defer"] = true
	}
	if s.Async {
		m["async"] = true
	}
	if s.Integrity != "" {
		m["integrity"] = s.Integrity
	}
	if s.Crossorigin != "" {
		m["crossorigin"] = s.Crossorigin
	}
	if s.Referrerpolicy != "" {
		m["referrerpolicy"] = s.Referrerpolicy
	}
	for k, v := range s.Data {
		m[k] = v
	}
	return json.Marshal(m)
}

// PrintJSON writes the resolved config as indented JSON to w (line-number
// fields excluded). Milestone 1's build output; milestone 2 replaces it with
// the real manifest print.
func PrintJSON(w io.Writer, c *Config) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}
