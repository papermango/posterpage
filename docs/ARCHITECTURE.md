# posterpage — Architecture & Maintenance

How `posterpage` is built: package structure, design decisions, recurring code patterns,
and how to build, use, and maintain it.

Companion docs: [BUILD_PLAN.md](BUILD_PLAN.md) is the authoritative spec (config schema,
validation rules, template details); [README.md](../README.md) is the quick-start.

---

## 1. What it is

A single Go binary that compiles a **config directory** (one `site.yml`/`.yaml`/`.json`,
one stylesheet, content files) into a minimal, self-contained static site:
`index.html` + `styles.css` + copied images. There is **no runtime JavaScript** of our own —
the only runtime script ever emitted is an SSG-generated constant for `color_mode: auto`.

The design constraints that shaped everything:
only **one external Go dependency** (`go.yaml.in/yaml/v3`; everything else stdlib),
minimal lines of code (~1,700 LOC + ~1,250 test LOC), minimal shipped CSS (~30 kB raw /
~6 kB gzip budget, met by Bootstrap partial-tree-shaking at the stylesheet level).

## 2. Package structure

```
posterpage/
├── cmd/posterpage/main.go          CLI: flag parsing only; exits 1 with
│                                   `posterpage: <error>` on stderr for any failure.
├── internal/
│   ├── config/
│   │   ├── config.go               Schema structs (Site/Head/Fonts/Links…) with
│   │   │                           yaml+json tags; per-type UnmarshalYAML capturing
│   │   │                           source line numbers; unknown-key rejection helpers.
│   │   ├── load.go                 Finds exactly one config file, decodes by
│   │   │                           extension, applies defaults (fonts.display →
│   │   │                           swap, site.link_style → button, site.color_mode → light).
│   │   └── validate.go             All allowlist/semantic checks; collects EVERY
│   │                               problem into one error, rendered as
│   │                               `file:line: path: message`.
│   ├── build/
│   │   ├── build.go                Pipeline orchestration + manifest + human sizes.
│   │   ├── template.go             Render Input assembly (NavLinks resolution,
│   │   │                           HeadTags, ColorScript) + writing index.html.
│   │   ├── styles.go               Styles step: styles.css passthrough or
│   │   │                           `sass` execution (errors carry the stderr tail,
│   │   │                           capped at 20 lines).
│   │   ├── head.go                 Flattens og/twitter + fonts + user head config
│   │   │                           into ordered, emit-ready headTag values;
│   │   │                           builds the Google Fonts css2 URL.
│   │   └── assets.go               Verbatim copies of picture/favicon (+ self-copy guard).
│   └── (tests live beside the code: *_test.go)
├── templates/
│   ├── index.gohtml                The one page template + htag/hattr/scriptel defines.
│   └── templates.go                `//go:embed index.gohtml`; small root-level package.
├── assets/
│   ├── bootstrap/                  Vendored Bootstrap 5.3.8 scss (92 files) + VERSION + LICENSE.
│   └── assets.go                   `//go:embed all:bootstrap` + Extract(dest) to a temp dir.
├── example/                        A complete, buildable content dir (site.yml,
│   │                               styles.scss, me.jpg, favicon.png).
├── docs/BUILD_PLAN.md              Spec: schema, validation, template, pipeline, milestones.
├── docs/ARCHITECTURE.md            This file.
└── README.md                       Quick-start.
```

Test conventions live in `config_test.go` (table-driven fixtures in `t.TempDir()`),
`head_test.go` (fonts URL, tag flattening, escaping), `styles_test.go` (passthrough,
compile, missing-sass), `build_test.go` (byte-exact golden HTML, pipeline, manifest).

## 3. Build pipeline (`posterpage build`)

`build.Build(cfgDir, outDir)` runs, in order:

1. `config.Load` — one config file, strict decode (yaml `KnownFields(true)`, json
   `DisallowUnknownFields()`), defaults applied.
2. `config.Validate` — schema + allowlists + referenced-file existence; the styles
   check (exactly one of `styles.scss`/`styles.css`) happens here.
3. `os.MkdirAll(outDir)`.
4. **Styles step** (`buildStyles`) — runs *before* render so a sass failure leaves no
   half-written output. `.css` passes through byte-for-byte; `.scss` runs
   `sass --quiet --style=compressed --no-source-map --load-path=<extracted vendored scss>`
   (vendored scss is extracted from the embedded archive to a fresh temp dir per run,
   so an installed binary works without the source checkout).
   Manifest order is stable: `index.html, styles.css, picture, favicon`.
5. **Render** (`writeIndex`) — Input = config + resolved NavLinks + HeadTags (og block →
   fonts block → user head tags) + color-mode script; executes `templates/index.gohtml`.
6. **Assets** (`copyAssets`) — picture/favicon verbatim; skipped when out dir == config dir.
7. `PrintManifest` — `name size` lines (KiB-labeled 1024 steps) + `total`.

Cwd never matters: all paths handed to sass are absolute; the config/out dirs come
explicitly from flags.

## 4. Config → what it controls

Authoritative schema: BUILD_PLAN.md §"Config schema". Summary:

| Config | Effect |
|---|---|
| `site.title/description/bio` | `<title>`, meta description, visible bio; og:title/og:description. |
| `site.picture` / `favicon` | copied verbatim; `<img>` + `<link rel="icon">`; og:image. |
| `site.url` | makes og:url + og:image absolute (otherwise og:image stays relative). |
| `site.color_mode` | `light` (nothing) / `dark` (data-bs-theme attr) / `auto` (attr + first-in-head constant script following prefers-color-scheme). |
| `site.link_style` + `links[].style` | nav entry render: `button` (Bootstrap outline btn) or `link` (plain anchor styled by the user's stylesheet). Per-entry overrides global; mixed lists legal. |
| `head.meta/links/scripts` | extra tags, allowlist-validated (scripts src-only, no inline bodies; `data-*` keys regex-checked). |
| `fonts` | generates the two preconnects + css2 stylesheet URL (families in listed order, spaces→`+`, weights `:wght@…`, `&display=` always). |

Duplicating the fonts stylesheet via `head.links` is a validation error.

## 5. Design decisions & why

- **Bootstrap 5.3.8, vendored and embedded.** 6 was rejected (alpha-only; 5 already dropped
  jQuery). The scss is checked in under `assets/bootstrap/` and compiled via `go:embed`,
  so the binary needs no package manager, no network, no repo-relative paths.
- **Tree-shaking lives in the user's `example/styles.scss`.** Go never parses CSS; the
  stylesheet is compiled "as-is". Size control comes from *which partials* are imported
  and *how* the `$theme-colors`/`$utilities` maps are narrowed to exactly the classes
  the template uses — that's the pattern when adding markup (see §6).
- **sass is shelled out with `--quiet`**, not a surgical `--silence-deprecation` list:
  Bootstrap 5.3.8 trips four deprecation families on dart-sass 1.106; a fixed list goes
  stale with every release. Build errors still surface (nonzero exit + stderr tail).
- **html/template everywhere, no manual escaping.** Attribute *values* are plain strings
  under auto-escaping. Attribute *names* are typed `template.HTMLAttr` — required because
  html/template never interpolates a plain string into attr-name position verbatim
  (`ZgotmplZ`); this is sound only because names are a closed identifier set or
  `data-*` keys validated to `^data-[a-z0-9_-]+$`. Keep that property if tags grow.
- **Dark mode costs zero CSS** because 5.3's `root` partial already compiled both color
  modes into the trimmed stylesheet — dark/auto are pure attribute/script toggles.
- **YAML first-class, JSON accepted** — both decode into one struct; JSON configs have
  no line numbers (structural errors only).
- **Deterministic output.** Map iteration is randomized in Go, so anything coming from a
  map (script `data-*` keys) is sorted; manifest order is fixed; golden tests compare
  HTML byte-for-byte.

## 6. Codebase patterns (how to do the common things)

**Add a config field** — five touchpoints, all mirrored in this order:
1. struct field in `internal/config/config.go` (yaml+json tags; unknown-key rejection
   picks new names up automatically via `keySet`);
2. default in `internal/config/load.go` if it needs one;
3. a validation bullet in `internal/config/validate.go` (follow the `problem`/line-number
   pattern);
4. use it in `internal/build` (and the template);
5. tests: fixture cases in `config_test.go`; extend the build golden if it affects HTML.
Then update BUILD_PLAN.md's schema section (it is the living spec).

**Add a built-in head tag** — extend `ogTags`/`builtInHeadTags` in `internal/build/head.go`
(keep the ordering contract documented on `headTags`), extend the goldens. Never reuse the
user-config meta machinery for built-ins — they're exempt from the meta allowlist by design.

**Use a new Bootstrap class in the template** — the stylesheet must grow with the markup:
add the utility key to `$keep` in `example/styles.scss`, and if the utility emits far more
than needed (e.g. `text-align` responsive variants), define a narrowed value in the
`$utilities: map.merge(...)` block *before* filtering. After building, grep
`styles.css` to confirm a rule exists for every class you used, and check the raw
size stays near the ~30 kB budget (handy: `gzip -c styles.css | wc -c`).

**Validation errors** — always via the collected `ps []problem` list with
`file:line: path: message` rendering; never fail-fast, so one build shows all problems.

**Source-line capture** — any type whose validation reports line numbers has
`UnmarshalYAML(n *yaml.Node)` storing `n.Line` via the
`type raw T; n.Decode((*raw)(t))` alias pattern (see `config.go`). Tree-like errors
must keep working when decoded through those types.

**Tests** — config fixtures are written per-case into `t.TempDir()`; goldens are inline
expected strings (byte-exact, comments explain each section). Sass-dependent tests skip
gracefully when the binary is absent.

## 7. Building, running, testing

```sh
brew install sass/sass/sass          # only for styles.scss configs (styles.css skips it)
go mod tidy && go build ./...        # regenerate vendored module sums, compile
go vet ./... && go test ./... -count=1   # the standard gate
go run ./cmd/posterpage build -c example -o public
# manifest on stdout; output overwritten on rebuild, nothing written outside -o
```

Every milestone/maintenance commit runs the full gate before committing. Golden tests
make template regressions loud; config table tests make validation regressions loud.

## 8. Maintenance playbooks

- **Bootstrap upgrade (5.3 → 6.x):** replace `assets/bootstrap/` contents from the new
  release's tarball (npm pack or GitHub), bump `assets/bootstrap/VERSION`, then work through
  `example/styles.scss` — v6 drops `@import` for `@use`, so partial lists map to module
  imports, `$theme-colors`/`$utilities` narrowing needs re-review, and the goldens/tests
  will catch markup drift. BUILD_PLAN.md tracks the approved version rationale.
- **Dependency updates:** the only external module is `go.yaml.in/yaml/v3`;
  `go mod tidy` + the full gate. The pre-ngo archive (`gopkg.in/yaml.v3`) must not be
  reintroduced.
- **Existing-tooling drift:** when dart-sass changes semantics (new deprecations, exit
  codes), the fail path in `buildStyles` carries them through the stderr tail — treat CI
  style errors as a styles.go reading, not config.
- **Authoring your real site:** copy `example/` → e.g. `site/`, drop in a real picture
  (same `site.picture` name), fill the real bio/links/twitter domain, set `site.url` when
  deploying so og URLs resolve. Build, then deploy `-o` output anywhere static
  (any object store + CDN, GH Pages, rsync + nginx); point DNS there.
- **Deploying:** output is plain static files with relative references (styles.css,
  favicon, picture), so no special host config is needed; the og absolute URLs depend on
  `site.url` being correct.
- **Size budget:** if a change pushes raw CSS beyond ~30 kB, remove classes from the
  template or narrow `$utilities` values further — never widen the import list to cover
  unreachable variants.
