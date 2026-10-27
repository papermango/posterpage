# posterpage

**posterpage** is a static site generator and the site it builds: a minimal, responsive,
list-of-links personal page (links, short bio, picture) — a single-page "poster" layout.
Optimized for minimal dependencies, minimal shipped bytes, and minimal source code.

This repo builds website v3; the full spec lives in [docs/BUILD_PLAN.md](docs/BUILD_PLAN.md)
(config schema, validation rules, HTML template, styles pipeline), and architecture /
maintenance notes live in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Usage

- Go 1.2x toolchain (`go build ./...`); a working site dir (`example/` is one).
- Sass compiles `styles.scss` (Bootstrap partial tree-shake): `brew install sass/sass/sass`.
  A plain `styles.css` passes through untouched, so Sass is optional.

```sh
go build ./cmd/posterpage
./posterpage build -c example -o public
# defaults: -c ./site, -o ./public
```

Config dir layout: `example/`.

## Configuration

One file, `site.yml` (or `.yaml`/`.json` — same structure), with sections:

- `site:` — `title`, `description`, `bio`, `picture` (required), `favicon`, `link_style`
  (`button | link`), `url` (absolute site URL → `og:url` + absolute `og:image`),
  `color_mode` (`light | dark | auto`).
- `head:` — optional extra `meta`, `links`, `scripts` entries, validated against allowlists.
- `fonts:` — Google Fonts shortcut; generates preconnects + the `css2` stylesheet link.
- `links:` — the nav entries, in order; `subtitle` as a second line, per-entry `style`.

Full schema, defaults, and validation rules: [docs/BUILD_PLAN.md](docs/BUILD_PLAN.md)
"Config schema" section.

## Output

`posterpage build` emits `index.html`, `styles.css` (sass-compressed or passthrough), and
copies `picture` / `favicon` verbatim, printing a manifest. Current `example/` build:

| File | Raw | Gzip |
|---|---|---|
| index.html | 1,750 B | 811 B |
| styles.css | 29,747 B | 5,825 B |
| me.jpg | 4,193 B | — |
| favicon.png | 103 B | — |

CSS sits inside the plan's budget (~30 kB raw / ~6 kB gzip); images are the user's choice.
