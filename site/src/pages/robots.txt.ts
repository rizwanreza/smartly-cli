import type { APIRoute } from 'astro';
import { withBase } from '../lib/url';

export const GET: APIRoute = ({ site }) => {
  const sitemap = new URL(withBase('/sitemap-index.xml'), site).href;
  return new Response(`User-agent: *\nAllow: /\n\nSitemap: ${sitemap}\n`, {
    headers: { 'content-type': 'text/plain; charset=utf-8' },
  });
};
