# smartly website

Landing page, documentation and brand guideline for the `smartly` CLI, published
to GitHub Pages at <https://rizwanreza.github.io/smartly-cli/>.

Astro, static output, no client framework, no analytics, no third-party font
requests.

## Local development

```bash
cd site
npm install
npm run dev      # http://localhost:4321/smartly-cli/
```

The dev server serves under the project base path by default, so local URLs
match production. To develop at the domain root instead:

```bash
BASE_PATH=/ npm run dev
```

## Production build

```bash
npm run build          # astro build + link validation, base path /smartly-cli
npm run build:root     # the same, with base path /
npm run preview        # serve the built dist/
```

`npm run build` fails if any internal link, heading fragment or asset is broken,
or if an internal URL is missing the configured base path.

To simulate a project-pages deploy exactly, serve `dist/` from a `smartly-cli/`
sub-directory:

```bash
npm run build
mkdir -p /tmp/pages && ln -sfn "$PWD/dist" /tmp/pages/smartly-cli
python3 -m http.server 8080 --directory /tmp/pages
# http://localhost:8080/smartly-cli/
```

## Base path handling

Every deployment-shaped value lives in [`site.config.mjs`](./site.config.mjs).
`BASE_PATH` (default `/smartly-cli`) feeds three things:

1. Astro's `base` option, which prefixes generated routes and bundled assets.
2. `withBase()` in [`src/lib/url.ts`](./src/lib/url.ts), used by every component
   that writes an internal URL.
3. [`scripts/rehype-base-urls.mjs`](./scripts/rehype-base-urls.mjs), which
   rewrites root-relative links written inside Markdown.

The Pages workflow passes `${{ steps.pages.outputs.base_path }}`, so the
repository name is never hard-coded — renaming the repo or moving to a user page
needs no source change.

## Structure

```
site/
├── astro.config.mjs        Astro config; Shiki theme and rehype plugins
├── site.config.mjs         BASE_PATH, SITE_URL, repo URLs, code theme
├── scripts/
│   ├── build-assets.mjs    Generates og.png, apple-touch-icon.png, favicon.ico
│   ├── check-links.mjs     Post-build link, fragment and base-path validation
│   ├── rehype-base-urls.mjs
│   ├── rehype-heading-anchors.mjs
│   └── rehype-wrap-tables.mjs
├── public/                 Static, unhashed: favicons, og.png, brand assets
└── src/
    ├── assets/fonts/       Self-hosted woff2 (hashed at build time)
    ├── assets/fonts-ttf/   Latin-subset TTFs, used only by build-assets.mjs
    ├── components/         Header, Sidebar, search, tabs, callouts, demo
    ├── content/docs/       Documentation, one MDX file per page
    ├── layouts/            Base and DocLayout
    ├── lib/                nav.ts (information architecture), url.ts
    ├── pages/              index, brand, 404, docs route, robots, search index
    ├── scripts/site.ts     All client behaviour, ~7 kB
    └── styles/             tokens.css, global.css, prose.css
```

## Adding a documentation page

1. Add `src/content/docs/<slug>.mdx` with `title`, `description`, `eyebrow` and
   `lede` frontmatter.
2. Add the slug to a group in [`src/lib/nav.ts`](./src/lib/nav.ts). That drives
   the sidebar, the drawer, previous/next and the search index.

Documentation content must match the repository `README.md`, which is the source
of truth for product behaviour.

## Regenerating raster brand assets

`public/og.png`, `public/apple-touch-icon.png` and `public/favicon.ico` are
committed and only need rebuilding when the source SVGs or the Open Graph copy
change:

```bash
npm install            # includes the @resvg/resvg-js devDependency
npm run build:assets
```

CI installs with `--omit=dev` and never runs this step.

## Fonts

Instrument Sans and Geist Mono are self-hosted as woff2 in `src/assets/fonts`,
under the SIL Open Font License; the licence texts sit beside them. The TTFs in
`src/assets/fonts-ttf` exist only so the asset script can rasterise text — they
are latin-subset, and the Instrument Sans one is a static wght=600 instance,
because the renderer ignores font-weight on a variable font.

## Deployment

[`.github/workflows/pages.yml`](../.github/workflows/pages.yml) builds and
deploys on every push to `main` that touches `site/`. Enable Pages for the
repository with **Settings → Pages → Source: GitHub Actions** once.
