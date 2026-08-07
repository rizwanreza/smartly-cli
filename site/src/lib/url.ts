/**
 * Base-path helpers.
 *
 * `import.meta.env.BASE_URL` is whatever Astro derived from `base` in
 * astro.config.mjs, which in turn comes from the BASE_PATH environment
 * variable. Astro is not perfectly consistent about the trailing slash across
 * versions, so everything is normalised here once and every internal URL in the
 * site goes through `withBase()`.
 */

const RAW = import.meta.env.BASE_URL || '/';

/** '' for a root deploy, '/smartly-cli' for project pages. */
export const BASE = RAW === '/' ? '' : RAW.replace(/\/+$/, '');

const ABSOLUTE = /^(?:[a-z][a-z0-9+.-]*:|\/\/|#|\?)/i;

/** Prefix a root-relative path with the deployment base path. */
export function withBase(path: string): string {
  if (ABSOLUTE.test(path)) return path;
  const clean = path.startsWith('/') ? path : `/${path}`;
  return `${BASE}${clean}` || '/';
}

/** True when `href` is the current page (or an ancestor of it, if `nested`). */
export function isActive(
  currentPathname: string,
  href: string,
  nested = false,
): boolean {
  const norm = (v: string) => (v.endsWith('/') ? v : `${v}/`);
  const current = norm(currentPathname);
  const target = norm(href);
  return nested ? current.startsWith(target) : current === target;
}
