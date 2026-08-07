// Single source of truth for deployment-shaped config.
//
// Base path handling
// ------------------
// The site is served from the apex of smartlycli.com, so the base is `/`. It is
// still read from BASE_PATH rather than hard-coded, because the sub-path case is
// one DNS change away from being real again: GitHub *project* pages live at
// https://<user>.github.io/<repo>/, where every internal URL must carry a
// `/smartly-cli` prefix. The Pages workflow passes
// `${{ steps.pages.outputs.base_path }}`, which follows whatever the Pages site
// is actually configured as, and `npm run build:subpath` exercises the prefixed
// case so that machinery cannot rot unnoticed.
//
// Everything downstream derives from this: Astro's `base` option, the
// `withBase()` helper used by components, and the rehype plugin that rewrites
// root-relative links written inside Markdown.

const rawBase = process.env.BASE_PATH ?? '/';

/** Normalised to either '/' or '/segment' (leading slash, no trailing slash). */
export const BASE_PATH = normaliseBase(rawBase);

/** Origin only — Astro joins this with `base` to build canonical URLs. */
export const SITE_URL = process.env.SITE_URL ?? 'https://smartlycli.com';

export const REPO_URL = 'https://github.com/rizwanreza/smartly-cli';
export const REPO_EDIT_URL = `${REPO_URL}/edit/main/site`;

function normaliseBase(value) {
  let base = String(value || '/').trim();
  if (!base.startsWith('/')) base = `/${base}`;
  base = base.replace(/\/+$/, '');
  return base === '' ? '/' : base;
}

/**
 * Shiki theme, derived from the Smartly palette rather than a stock theme.
 * Four roles only: ink for commands, muted grey for comments and punctuation,
 * deep cyan for strings and paths, dark amber for flags and numbers. Every
 * foreground clears 4.5:1 against warm paper (#F4F1E8).
 */
export const shikiTheme = {
  name: 'smartly-paper',
  type: 'light',
  colors: {
    'editor.background': '#F4F1E8',
    'editor.foreground': '#151716',
  },
  settings: [
    { settings: { background: '#F4F1E8', foreground: '#151716' } },
    {
      scope: ['comment', 'punctuation.definition.comment', 'string.comment'],
      settings: { foreground: '#5F6461', fontStyle: 'italic' },
    },
    {
      scope: [
        'string',
        'string.quoted',
        'punctuation.definition.string',
        'meta.attribute string',
      ],
      settings: { foreground: '#006473' },
    },
    {
      scope: [
        'constant.numeric',
        'constant.language',
        'constant.other.option',
        'variable.parameter',
        'entity.name.tag',
        'support.type.property-name',
      ],
      settings: { foreground: '#8A5200' },
    },
    {
      scope: [
        'keyword',
        'keyword.operator',
        'storage',
        'storage.type',
        'entity.name.function',
        'support.function',
        'variable.function',
      ],
      settings: { foreground: '#151716', fontStyle: 'bold' },
    },
    {
      scope: [
        'punctuation',
        'meta.brace',
        'variable.other',
        'variable.language',
        'punctuation.separator',
      ],
      settings: { foreground: '#5F6461' },
    },
  ],
};
