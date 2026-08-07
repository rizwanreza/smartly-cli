#!/usr/bin/env node
/**
 * Link validation for the built site.
 *
 * Walks every HTML file in dist/ and checks that:
 *   1. every internal href resolves to a real file (or directory index) in dist;
 *   2. every fragment points at an id that actually exists on the target page;
 *   3. every internal src (images, fonts, scripts, stylesheets) exists;
 *   4. every internal URL carries the configured base path — the single most
 *      likely way to break a GitHub project-pages deploy.
 *
 * External URLs are listed but not fetched, so the check stays offline and
 * deterministic. Run it against both base paths:
 *
 *   npm run build          # /smartly-cli/
 *   npm run build:root     # /
 */

import { readdir, readFile } from 'node:fs/promises';
import { existsSync, statSync } from 'node:fs';
import { join, relative, posix } from 'node:path';
import { fileURLToPath } from 'node:url';

import { BASE_PATH } from '../site.config.mjs';

const root = fileURLToPath(new URL('..', import.meta.url));
const dist = join(root, 'dist');
const prefix = BASE_PATH === '/' ? '' : BASE_PATH;

if (!existsSync(dist)) {
  console.error('check-links: dist/ not found. Run the build first.');
  process.exit(1);
}

const htmlFiles = await walk(dist, '.html');
const ids = new Map(); // route -> Set<id>
const pages = new Map(); // route -> { file, links, assets }
const external = new Set();
const errors = [];

for (const file of htmlFiles) {
  const html = await readFile(file, 'utf8');
  const route = toRoute(file);
  ids.set(route, collectIds(html));
  pages.set(route, {
    file: relative(root, file),
    links: collect(html, /<a\b[^>]*?\bhref="([^"]*)"/gi),
    assets: [
      ...collect(html, /<(?:img|script|source)\b[^>]*?\bsrc="([^"]*)"/gi),
      ...collect(html, /<link\b[^>]*?\bhref="([^"]*)"/gi),
    ],
  });
}

for (const [route, page] of pages) {
  for (const raw of page.links) {
    checkUrl(raw, route, page, true);
  }
  for (const raw of page.assets) {
    checkUrl(raw, route, page, false);
  }
}

await checkSearchIndex();

report();

/** The search index links straight to headings, so its URLs need the same
 *  guarantees as the ones written into the HTML. */
async function checkSearchIndex() {
  const file = join(dist, 'search-index.json');
  if (!existsSync(file)) {
    errors.push('search-index.json was not generated');
    return;
  }
  const records = JSON.parse(await readFile(file, 'utf8'));
  for (const record of records) {
    checkUrl(record.url, '/', { file: 'search-index.json', links: [], assets: [] }, true);
  }
  console.log(`check-links: search index has ${records.length} records`);
}

/* ------------------------------------------------------------------ check */

function checkUrl(raw, route, page, isLink) {
  const url = raw.trim();
  if (!url || url.startsWith('mailto:') || url.startsWith('tel:')) return;

  if (/^(?:[a-z][a-z0-9+.-]*:)?\/\//i.test(url)) {
    external.add(url.split('#')[0]);
    return;
  }

  if (url.startsWith('#')) {
    const id = decodeURIComponent(url.slice(1));
    if (id && !ids.get(route)?.has(id)) {
      errors.push(`${page.file}: fragment ${url} has no matching id on the page`);
    }
    return;
  }

  const [pathPart, hash] = url.split('#');
  const target = pathPart.startsWith('/')
    ? pathPart
    : posix.join(posix.dirname(route === '/' ? '/index' : route), pathPart);

  // Base-path guard: internal absolute URLs must carry the configured prefix.
  if (prefix && pathPart.startsWith('/') && !pathPart.startsWith(`${prefix}/`) && pathPart !== prefix) {
    errors.push(
      `${page.file}: ${url} is missing the base path "${prefix}" — it would 404 on project pages`,
    );
    return;
  }

  const resolved = resolveFile(target);
  if (!resolved) {
    errors.push(`${page.file}: ${isLink ? 'link' : 'asset'} ${url} does not exist in dist/`);
    return;
  }

  if (hash && resolved.route) {
    const id = decodeURIComponent(hash);
    if (!ids.get(resolved.route)?.has(id)) {
      errors.push(`${page.file}: ${url} points at an id that does not exist on ${resolved.route}`);
    }
  }
}

/** Map a site path to a file in dist/, honouring directory-style routes. */
function resolveFile(sitePath) {
  const withoutPrefix =
    prefix && sitePath.startsWith(prefix)
      ? sitePath.slice(prefix.length) || '/'
      : sitePath;
  const clean = decodeURIComponent(withoutPrefix);

  const candidates = [
    join(dist, clean),
    join(dist, clean, 'index.html'),
    join(dist, `${clean}.html`),
  ];

  for (const candidate of candidates) {
    if (!existsSync(candidate)) continue;
    if (statSync(candidate).isDirectory()) continue; // a bare directory is a 404
    return {
      file: candidate,
      route: candidate.endsWith('.html') ? toRoute(candidate) : null,
    };
  }
  return null;
}

/* ------------------------------------------------------------------ utils */

async function walk(dir, ext) {
  const out = [];
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...(await walk(full, ext)));
    else if (entry.name.endsWith(ext)) out.push(full);
  }
  return out;
}

function toRoute(file) {
  const rel = relative(dist, file).split('\\').join('/');
  const route = rel.endsWith('index.html') ? rel.slice(0, -'index.html'.length) : rel;
  return `/${route}`.replace(/\/+$/, '/') || '/';
}

function collect(html, pattern) {
  return [...html.matchAll(pattern)].map((match) => match[1]);
}

function collectIds(html) {
  const set = new Set();
  for (const match of html.matchAll(/\bid="([^"]+)"/g)) set.add(match[1]);
  for (const match of html.matchAll(/\bname="([^"]+)"/g)) set.add(match[1]);
  return set;
}

function report() {
  const pageCount = pages.size;
  const linkCount = [...pages.values()].reduce(
    (total, page) => total + page.links.length + page.assets.length,
    0,
  );

  console.log(
    `check-links: base "${BASE_PATH}" · ${pageCount} pages · ${linkCount} internal/external references · ${external.size} distinct external URLs (not fetched)`,
  );

  if (errors.length) {
    console.error(`\n${errors.length} broken reference(s):\n`);
    for (const error of errors) console.error(`  ✗ ${error}`);
    process.exit(1);
  }

  console.log('check-links: no broken internal links, fragments or assets.');
}
