/**
 * Rewrites root-relative URLs inside Markdown/MDX output so they survive being
 * served from a GitHub project-pages sub-path.
 *
 * Authors write `/docs/usage/` in content; this turns it into
 * `/smartly-cli/docs/usage/` at build time. Absolute URLs, protocol-relative
 * URLs, fragments and mailto: links are left alone. Zero dependencies — it is a
 * plain recursive walk over the hast tree.
 */

const ATTRS = ['href', 'src', 'poster'];
const EXTERNAL = /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i;

export function rehypeBaseUrls({ base = '/' } = {}) {
  const prefix = base === '/' ? '' : base.replace(/\/+$/, '');

  return function transformer(tree) {
    if (!prefix) return;
    walk(tree);
  };

  function walk(node) {
    if (!node || typeof node !== 'object') return;
    if (node.type === 'element' && node.properties) {
      for (const attr of ATTRS) {
        const value = node.properties[attr];
        if (typeof value !== 'string') continue;
        if (!value.startsWith('/') || value.startsWith('//')) continue;
        if (EXTERNAL.test(value)) continue;
        node.properties[attr] = prefix + value;
      }
    }
    if (Array.isArray(node.children)) {
      for (const child of node.children) walk(child);
    }
  }
}
