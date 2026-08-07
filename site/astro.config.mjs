import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';

import { BASE_PATH, SITE_URL, shikiTheme } from './site.config.mjs';
import { rehypeBaseUrls } from './scripts/rehype-base-urls.mjs';
import { rehypeHeadingAnchors } from './scripts/rehype-heading-anchors.mjs';
import { rehypeWrapTables } from './scripts/rehype-wrap-tables.mjs';

// https://astro.build/config
export default defineConfig({
  site: SITE_URL,
  base: BASE_PATH,
  trailingSlash: 'always',
  build: { format: 'directory' },
  integrations: [mdx(), sitemap()],
  markdown: {
    syntaxHighlight: 'shiki',
    shikiConfig: {
      theme: shikiTheme,
      wrap: false,
    },
    rehypePlugins: [
      rehypeHeadingAnchors,
      rehypeWrapTables,
      [rehypeBaseUrls, { base: BASE_PATH }],
    ],
  },
  vite: {
    build: {
      assetsInlineLimit: 0,
    },
  },
});
