package build

import (
	"html/template"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"posterpage/internal/config"
)

// headAttr is one attribute of an emitted head tag: `key="value"` when Value
// is set, the bare attribute `key` when Value is empty — which is exactly how
// defer / async / crossorigin render.
//
// Key is typed template.HTMLAttr because html/template never emits an action's
// output in attribute-name position verbatim: any plain string there is
// replaced by "ZgotmplZ" (see htmlNameFilter in html/template/html.go;
// HTMLAttr is the designated safe-attribute-name content type). This is sound
// here because every key headTags emits is either one of the fixed
// identifiers below or a config data-* key, which validation restricts to
// ^data-[a-z0-9_-]+$.
//
// Attribute values are plain strings and stay under html/template's
// auto-escaping; nothing is hand-escaped.
type headAttr struct {
	Key   template.HTMLAttr
	Value string
}

// bare returns the key as a bare attribute (defer, async, crossorigin).
func bare(key string) headAttr {
	return headAttr{Key: template.HTMLAttr(key)}
}

// attr returns a key="value" attribute.
func attr(key, value string) headAttr {
	return headAttr{Key: template.HTMLAttr(key), Value: value}
}

// headTag is one emit-ready tag for templates/index.gohtml. Name is one of
// "meta", "link", "script" — the template picks the literal tag text and the
// void-vs-script closing from the name, so the set must not grow without a
// template change. Attr order is the emit order, and must be built
// deterministically in Go (headTags guarantees it).
type headTag struct {
	Name string
	Attr []headAttr
}

// scriptEl is one emit-ready <script> with an inline body: the only
// inline-script element posterpage emits, reserved for SSG-generated
// constants. head.scripts config entries are src-only by validation and can
// never produce one (nor should they — the inline prohibition covers them).
// Body is typed template.JS because the template define that renders scriptEl
// sits inside a script element context, where html/template would otherwise
// JS-escape the body; the value comes verbatim from this package's constant,
// so no escaping is wanted.
type scriptEl struct {
	Attr []headAttr
	Body template.JS
}

// colorModeScriptBody is the hand-formatted, deterministic script emitted as
// the FIRST element in <head> for site.color_mode: auto. It applies the
// visitor's prefers-color-scheme to Bootstrap's data-bs-theme before first
// paint, so an auto-mode page never flashes light first.
const colorModeScriptBody = template.JS(`
  const dark = window.matchMedia("(prefers-color-scheme: dark)");
  document.documentElement.dataset.bsTheme = dark.matches ? "dark" : "light";
`)

// colorModeScriptEl builds the emit-ready color-mode script element.
func colorModeScriptEl() scriptEl {
	return scriptEl{Body: colorModeScriptBody}
}

// ogTags builds the og/twitter basics always emitted for the page (derived
// from site:, never user head config). Each tag renders as
// <meta property="…" content="…">. Order mirrors docs/BUILD_PLAN.md:
//
//	og:title (site.title, when set) → og:type "website" →
//	og:description (when set) → og:url (when site.url is set) →
//	og:image (site.picture: relative when site.url is unset, absolute
//	site-joined when set) → twitter:card (summary_large_image with a
//	picture, else summary).
//
// These are built-in tags, not user config: the head.meta "at most one key"
// validation restriction does not apply to them.
func ogTags(cfg *config.Config) []headTag {
	var tags []headTag
	site := cfg.Site
	og := func(property, content string) {
		tags = append(tags, headTag{Name: "meta", Attr: []headAttr{
			attr("property", property),
			attr("content", content),
		}})
	}
	if site.Title != "" {
		og("og:title", site.Title)
	}
	og("og:type", "website")
	if site.Description != "" {
		og("og:description", site.Description)
	}
	if site.Url != "" {
		og("og:url", site.Url)
	}
	if site.Picture != "" {
		og("og:image", ogImageURL(site.Url, site.Picture))
	}
	card := "summary"
	if site.Picture != "" {
		card = "summary_large_image"
	}
	// Twitter/X consumes twitter:* via the name attribute (unlike og:*,
	// which is property=), so twitter:card emits name= per the documented
	// format.
	tags = append(tags, headTag{Name: "meta", Attr: []headAttr{
		attr("name", "twitter:card"),
		attr("content", card),
	}})
	return tags
}

// ogImageURL resolves the og:image URL: the picture as a relative URL when
// site.url is unset, joined onto the site URL as an absolute URL when set.
func ogImageURL(siteURL, picture string) string {
	if siteURL == "" {
		return picture
	}
	u, err := url.Parse(siteURL)
	if err != nil {
		return picture // shape-validated upstream; unreachable via Load
	}
	return u.JoinPath(picture).String()
}

// headTags flattens the built-in tags plus cfg into the ordered emit-ready
// head tags:
//
//  1. og/twitter basics, always (ogTags),
//  2. Google Fonts preconnects + stylesheet link, when cfg.Fonts is set,
//  3. cfg.Head.Meta entries,
//  4. cfg.Head.Links entries,
//  5. cfg.Head.Scripts entries.
//
// Tags render as void elements (self-unclosed <meta>/<link>) except scripts,
// which close with </script>.
func headTags(cfg *config.Config) []headTag {
	tags := ogTags(cfg)
	tags = append(tags, builtInHeadTags(cfg)...)
	return tags
}

// builtInHeadTags emits the "head-input" tags: the Google Fonts block (when
// fonts is configured) followed by cfg.Head.Meta, cfg.Head.Links,
// cfg.Head.Scripts — the tags whose shape comes from user config (validated
// against the docs/BUILD_PLAN.md allowlists).
func builtInHeadTags(cfg *config.Config) []headTag {
	var tags []headTag
	if cfg.Fonts != nil {
		tags = append(tags,
			headTag{Name: "link", Attr: []headAttr{
				attr("rel", "preconnect"),
				attr("href", "https://fonts.googleapis.com"),
			}},
			headTag{Name: "link", Attr: []headAttr{
				attr("rel", "preconnect"),
				attr("href", "https://fonts.gstatic.com"),
				bare("crossorigin"),
			}},
			headTag{Name: "link", Attr: []headAttr{
				attr("rel", "stylesheet"),
				attr("href", fontsURL(cfg.Fonts)),
			}},
		)
	}
	for _, m := range cfg.Head.Meta {
		tags = append(tags, metaTag(m))
	}
	for _, l := range cfg.Head.Links {
		tags = append(tags, headTag{Name: "link", Attr: []headAttr{
			attr("rel", l.Rel),
			attr("href", l.Href),
		}})
	}
	for _, s := range cfg.Head.Scripts {
		tags = append(tags, scriptTag(s))
	}
	return tags
}

// metaTag converts one config meta entry. Validation guarantees that at most
// one of name/property/http-equiv/charset is set, and content is set unless
// charset is, so the render is: the one key pair, then content when present.
// A charset-only entry (the common casing) is therefore exactly one pair.
func metaTag(m config.MetaTag) headTag {
	t := headTag{Name: "meta"}
	switch {
	case m.Name != "":
		t.Attr = append(t.Attr, attr("name", m.Name))
	case m.Property != "":
		t.Attr = append(t.Attr, attr("property", m.Property))
	case m.HTTPEquiv != "":
		t.Attr = append(t.Attr, attr("http-equiv", m.HTTPEquiv))
	case m.Charset != "":
		t.Attr = append(t.Attr, attr("charset", m.Charset))
	}
	if m.Content != "" {
		t.Attr = append(t.Attr, attr("content", m.Content))
	}
	return t
}

// scriptTag converts one config script entry. Fixed attributes render as
// key="value" when set; defer/async as bare; data-* keys alphabetically (map
// iteration in Go is random, and output must be deterministic), each as
// bare-or-valued.
func scriptTag(s config.ScriptTag) headTag {
	t := headTag{Name: "script"}
	t.Attr = append(t.Attr, attr("src", s.Src))
	if s.Type != "" {
		t.Attr = append(t.Attr, attr("type", s.Type))
	}
	if s.Integrity != "" {
		t.Attr = append(t.Attr, attr("integrity", s.Integrity))
	}
	if s.Crossorigin != "" {
		t.Attr = append(t.Attr, attr("crossorigin", s.Crossorigin))
	}
	if s.Referrerpolicy != "" {
		t.Attr = append(t.Attr, attr("referrerpolicy", s.Referrerpolicy))
	}
	if s.Defer {
		t.Attr = append(t.Attr, bare("defer"))
	}
	if s.Async {
		t.Attr = append(t.Attr, bare("async"))
	}
	keys := make([]string, 0, len(s.Data))
	for k := range s.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := s.Data[k]
		if v == "" {
			t.Attr = append(t.Attr, bare(k))
		} else {
			t.Attr = append(t.Attr, attr(k, v))
		}
	}
	return t
}

// fontsURL builds the Google Fonts css2 URL for the fonts block:
//
//	https://fonts.googleapis.com/css2?family=Fam1[&family=Fam2…]&display=<display>
//
// Families render in listed order; spaces URL-encode as "+" (QueryEscape;
// family names are pre-validated to ^[A-Za-z0-9 ]+$). A family with weights
// appends :wght@W1;W2..., a family without renders plain. Display comes from
// the config and is defaulted to "swap" here too, defensively: Load already
// defaults it, but fontsURL must not depend on having been reached through
// Load.
func fontsURL(f *config.Fonts) string {
	display := f.Display
	if display == "" {
		display = "swap"
	}
	fams := make([]string, len(f.Families))
	for i, fam := range f.Families {
		q := "family=" + url.QueryEscape(fam.Family)
		if len(fam.Weights) > 0 {
			ws := make([]string, len(fam.Weights))
			for j, w := range fam.Weights {
				ws[j] = strconv.Itoa(w)
			}
			q += ":wght@" + strings.Join(ws, ";")
		}
		fams[i] = q
	}
	return "https://fonts.googleapis.com/css2?" + strings.Join(fams, "&") + "&display=" + display
}
