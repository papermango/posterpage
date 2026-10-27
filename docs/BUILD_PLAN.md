# Posterpage — Link-in-Bio Site + Generator (Build Plan)

**Project name: `posterpage`.** Maintained by Parth Deshmukh and agents. Successor to the
draft in `PLAN.md` at the repo root (the repo may also be renamed to `posterpage`).

## Abstract

**Posterpage** is a static site generator and the site it builds: a minimal, responsive,
list-of-links personal page (links, short bio, picture), named for the single-page "poster"
layout it produces. Optimized for: minimal dependencies, minimal shipped bytes, minimal
source code.

## Locked decisions

| Decision | Choice | Rationale |
|---|---|---|
| CSS framework | **Bootstrap 5.3.x** | Stable and already jQuery-free; 6.0.0-alpha.1 (Oct 2026) rejected — pre-stability. 5.3 is the documented migration base for v6 later. |
| Runtime JS framework | None (no React/Angular/Vue/jQuery) | Site is fully static; we ship no Bootstrap JS at all. |
| Config files | **One file**: `site.yml` | `head.yml` merged in as a `head:` section. One loader, one validation pass. |
| Config format | **YAML first-class**; `.json` accepted transparently | Both decode into the identical Go struct; no cross-compilation concept needed. |
| Google Fonts | **First-class `fonts:` key** → generates preconnect + stylesheet tags | Most common `head` use case; avoids hand-writing three tags and the `css2?` URL format by hand. Generics in `head.links` remain for everything else. |
| Styles pipeline | **Shell out to `sass` binary** (`dart-sass`) | No viable pure-Go Sass. `styles.css` passes through untouched, so Sass stays optional. |
| Template engine | Go `html/template` (stdlib) | Auto-escaping, zero deps. |
| Go dependencies | `go.yaml.in/yaml/v3` (only external import) | Original `gopkg.in/yaml.v3` is archived (Mar 2025); this is the community-maintained fork. All else stdlib. |
| Name | **`posterpage`** — applies to the Go module (`module posterpage`, canonicalized to `github.com/parthdeshmukh/posterpage` when published), the `posterpage` binary, and the repo | One name for the generator and the site. |

## Source layout

```
posterpage/
├── PLAN.md                      # original draft (superseded by this doc)
├── docs/BUILD_PLAN.md           # this file
├── go.mod
├── cmd/posterpage/main.go       # CLI entry: flag parsing, commands
├── internal/
│   ├── config/
│   │   ├── config.go            # struct + defaults
│   │   ├── load.go              # decode .yml/.yaml/.json by extension → struct
│   │   └── validate.go          # schema/allowlist validation, good error messages
│   ├── build/
│   │   ├── build.go             # pipeline orchestration
│   │   ├── template.go          # parse embedded template, execute with config
│   │   ├── styles.go            # sass invocation / css passthrough
│   │   └── assets.go            # copy picture + favicon
│   └── serve/serve.go           # optional: static fileserver + watch-rebuild
├── templates/
│   └── index.gohtml             # embedded via embed.FS
├── assets/
│   └── bootstrap/               # vendored Bootstrap 5.3 scss (sass --load-path target)
└── example/                     # example content dir (site.yml, styles.scss, me.jpg, favicon.png)
```

## Config schema — `site.yml` (first-class)

```yaml
site:
  title: Parth Deshmukh            # <title>, og:title
  description: Systems engineer …  # meta description, og:description
  bio: |                           # paragraph(s) shown under the picture
    Systems engineer. I build things.
  picture: me.jpg                  # filename in the config dir (required)
  favicon: favicon.png             # optional; emitted as <link rel="icon">
  link_style: button               # optional; button | link; default button (see links:)

head:                              # all optional; validated against allowlists
  meta:                            # extra <meta> tags
    - name: author
      content: Parth Deshmukh
  links:                           # extra <link> tags (fonts, preconnects)
    - rel: preconnect
      href: https://fonts.googleapis.com
    - rel: stylesheet
      href: https://fonts.googleapis.com/css2?family=Inter:wght@400;600&display=swap
  scripts:                         # <script> tags — analytics use case
    - src: https://plausible.io/js/script.js
      defer: true
      data-domain: parthdeshmukh.com

fonts:                             # Google Fonts shortcut (optional); generates the tags for you
  display: swap                    # optional font-display; default swap
  families:                        # rendered in the order listed
    - family: Inter
      weights: [400, 600]          # optional; omit for the family's default/variable range
    - family: JetBrains Mono

links:                             # the link list, rendered in order (required)
  - text: GitHub
    href: https://github.com/pdeshmukh
  - text: Blog
    href: https://parthdeshmukh.com/blog
    subtitle: Occasional writing    # optional second line under link text
    style: link                     # optional per-link override of site.link_style
```

`site.json` uses the exact same structure; loader picks decoder by extension. `.yml`/`.yaml`/`.json` all load; exactly one config file may exist per directory.

### Google Fonts

Given `fonts` in the config, Posterpage emits exactly these tags (before any `head` entries):

```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600&family=JetBrains+Mono&display=swap">
```

- Families are joined alphabetically stable in listed order as `family=F&family=G`; weights become `:wght@400;600`.
- No network request at build time — tags are linked, not fetched (keeps the build offline and hermetic). Self-hosting fonts locally is a future feature.
- If `fonts` is set *and* `head.links` contains a `fonts.googleapis.com` stylesheet, that's a duplicate → build error with a message pointing at whichever key to remove.

### Validation rules (head allowlist)

- `head.scripts[]` must have `src`; **inline script bodies are not allowed** (security + schema simplicity). Allowed attrs: `src, type, defer, async, integrity, crossorigin, referrerpolicy` plus `data-*` (regex `^data-[a-z0-9_-]+$`, string values).
- `head.links[]` `rel` must be one of: `stylesheet, preconnect, dns-prefetch, preload, icon, alternate`.
- `head.meta[]`: keys limited to `name | property | http-equiv | content | charset` (content required unless charset).
- `fonts.families[].family` must match `^[A-Za-z0-9 ]+$` (spaces URL-encoded as `+`); `weights[]` must be integers 100–900 in steps of 100 (variable ranges `100..1000` and `ital`/axes come later); `fonts.display` ∈ `auto | block | swap | fallback | optional` (default `swap`).
- Link render style: `site.link_style` and per-entry `links[].style` ∈ `button | link` (default `button`; a per-entry `style` overrides the global for that link). `button` renders the current Bootstrap outline button; `link` renders a plain, format-friendly `<a>` (styled entirely from `styles.scss` — that's the point of the mode).
- `href`s must parse as absolute https URLs or root-relative paths (`/…`); `http://` rejected with a warning.
- Referenced files (`picture`, `favicon`, styles) must exist in the config dir — validated before build.
- Errors report file + YAML line numbers.

## CLI

```
posterpage build [-c <config-dir>] [-o <out-dir>]    # defaults: ./site → ./public
posterpage serve [-o <out-dir>] [-addr :8080]        # serves out-dir; --watch triggers rebuild (milestone 7)
```

`build` prints each emitted file with its size and the total; writes nothing outside `-o`.

## HTML template (`templates/index.gohtml`)

Single page. Posterpage always emits: doctype, charset, viewport, title, description, favicon link,
stylesheet link, og/twitter basics derived from `site:`. Order: emitted basics → Google Fonts tags
(if `fonts` is configured) → config `head` entries.

`<meta name="viewport" content="width=device-width, initial-scale=1.0">` should always be included in `head`.

Body skeleton (Bootstrap classes only, no custom layout CSS):

```html
<body class="bg-body">
  <main class="container" style="max-width: 40rem">
    <header class="text-center">
      <img class="rounded-circle mt-5" src="…" width="120" height="120" alt="Parth Deshmukh">
      <h1 class="h3 mt-3">…</h1>
      <p class="text-body-secondary">…bio…</p>
    </header>
    <nav class="d-grid gap-2 my-4">
      <a class="btn btn-outline-secondary" href="…">GitHub</a>  <!-- per style: button mode, or a plain <a> for link mode; subtitle → <small> -->
    </nav>
  </main>
</body>
```

Responsive by construction: stacked column, container max-width, less work without the grid.
Zero JavaScript emitted (analytics excepted).

## Styles pipeline

- Config dir contains `styles.scss` or `styles.css` (not both; one is required).
- `.css` → copy verbatim to `public/styles.css`.
- `.scss` → run `sass --quiet --style=compressed --no-source-map --load-path=<vendored-scss-dir> styles.scss public/styles.css`; the load path is the vendored scss (checked in under `assets/bootstrap/`, embedded into the binary via `go:embed`) extracted to a fresh temp dir at build time, so an installed `posterpage` binary works without the source checkout nearby.
  - Missing `sass` binary → fail with a one-line install hint (`brew install sass/sass/sass`).
  - `--quiet` (not surgical `--silence-deprecation` lists) because dart-sass 1.106 flags several deprecation families in Bootstrap 5.3.8's `@import`-era scss; a fixed list goes stale with every sass release. Warnings are suppressed; compile errors still surface via a nonzero exit with the stderr tail.
- Posterpage never parses or alters stylesheet contents — "compiled and dropped in as-is" per original intent. Minification comes free from dart-sass.
- Recommended `example/styles.scss` (controls shipped size — importing partials is the tree-shake):

```scss
@use "sass:map";

@import "functions";
@import "variables";
@import "maps";
@import "mixins";

// Trimmed after @import "maps" (which finalizes $theme-colors): keep only the
// variant the links use (btn-outline-secondary).
$theme-colors: ("secondary": map.get($theme-colors, "secondary"));

// Before utilities/api generates CSS, keep only the utilities the markup uses.
$keep: ("background-color", "display", "gap", "margin-y", "rounded", "text-color");
@import "utilities";
$kept: ();
@each $u in $keep {
  $kept: map.merge($kept, ($u: map.get($utilities, $u)));
}
$utilities: $kept;

@import "root";
@import "reboot";
@import "type";      // .h1–.h6 heading classes (h3 in the page markup)
@import "buttons";
@import "containers";
@import "utilities/api";

// + user overrides below
:root { --bs-border-radius: 12px; }
```

This sample compiles to ~28 kB raw / ~5.7 kB gzipped — within the size budget. Extending the page markup means extending this file.

Bootstrap 5.3.8 scss is vendored into `assets/bootstrap/` (checked in, version documented in `assets/bootstrap/VERSION`) so no package manager is required; a `--load-path` override can support an npm workflow later.

## Build pipeline (`posterpage build`)

1. Locate & decode config (`site.yml` / `site.json`) → `Config` struct; validate.
2. Verify referenced files exist.
3. Styles step (compile or copy).
4. Execute template → `public/index.html`.
5. Copy `picture`, `favicon` verbatim (no re-encoding; stdlib-only constraint). Names sanitized (no path separators).
6. Print manifest + total output size.

Size budget: HTML ≤ 3 KB; CSS ≤ ~30 KB uncompressed / ~6 KB gzip (partial-import dependent); images unbounded (user's choice).

## Testing

- `internal/config`: table-driven tests over fixture files — valid yaml, valid json, each reject case of the allowlist (inline script, bad rel, disallowed attr, missing file, both-styles conflict).
- `internal/build`: golden-file test of template render (same config → byte-identical HTML).
- Styles step tested with `sass` present; skipped gracefully when absent in CI.
- `go vet ./...` in CI; manual smoke: build `example/`, serve, check phone + desktop, record gzipped sizes in README.

## Milestones (each ends buildable + committed)

1. **Skeleton**: `go.mod`, `posterpage build` CLI shell, config load/decode (yaml+json), validation. Deliverable: `posterpage build` parses `example/site.yml`, prints resolved config or a clean validation error.
2. **Page render without styles**: template, asset copy, manifest printing. Deliverable: unstyled but correct HTML.
3. **Styles pipeline**: vendored Bootstrap scss + sass exec + css passthrough. Deliverable: styled page in browser.
4. **Head injection**: meta/link/script emission with full allowlist validation, plus Google Fonts tag generation from `fonts:`. Deliverable: analytics script + Google Fonts verified in output.
5. **Polish**: favicon, og tags, dark mode (5.3 color modes + `prefers-color-scheme`), README (usage, schema doc, size report).
6. **Link render styles**: `site.link_style` global default + per-link `style:` override — nav links render either as today's Bootstrap buttons (`button`) or as plain, fully formattable `<a>` anchors (`link`, styled entirely by the user's `styles.scss`, exact classes chosen at implementation and verified in-browser). Extends types/template/validation already shipping; note in the milestone that extending markup = extending `example/styles.scss` (likely more utilities/partials). Deliverable: a `link`-styled entry renders correctly next to button entries.
7. **Optional**: `posterpage serve --watch`; later: Bootstrap 6 migration checklist once v6 is stable.

## Resolved former TBDs

- Schema: defined above.
- Consolidation: yes — single `site.yml`.
- YAML vs JSON: YAML first-class, JSON accepted; no cross-compilation — both decode to one struct.
- Bootstrap version: 5.3.x (corrected — v5 already dropped jQuery, so v6-alpha was unnecessary).

## Explicitly out of scope (v3)

MD/HTML content pages and blog rendering, Go template → CSS processing, image resizing/webp (revisit later), RSS, Google Fonts `ital`/variable-axes parameters and self-hosted fonts.
