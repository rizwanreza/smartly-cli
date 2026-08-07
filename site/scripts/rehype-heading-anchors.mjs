import GithubSlugger from 'github-slugger';

/**
 * Gives every h2/h3/h4 a stable id and a permalink control, so headings are
 * deep-linkable both by hand-written URL and by click.
 *
 * Two details that are easy to get wrong:
 *
 * 1. User rehype plugins run *before* Astro assigns heading ids, so this plugin
 *    assigns them itself with the same slugger Astro uses. Astro's own pass
 *    leaves an existing id alone, so the ids it reports in `headings` (which
 *    drive the table of contents) stay in sync with these.
 *
 * 2. The appended anchor contains no text. Astro derives the table-of-contents
 *    label from the heading's text content, so a literal "#" here would end up
 *    in every sidebar entry. The glyph is drawn by CSS instead.
 */

const TARGETS = new Set(['h2', 'h3', 'h4']);

export function rehypeHeadingAnchors() {
  return function transformer(tree) {
    const slugger = new GithubSlugger();
    walk(tree, slugger);
  };
}

function walk(node, slugger) {
  if (!node || typeof node !== 'object') return;

  if (node.type === 'element' && TARGETS.has(node.tagName)) {
    node.properties ??= {};
    const id = node.properties.id || slugger.slug(text(node));
    node.properties.id = id;
    node.properties.className = [
      ...toArray(node.properties.className),
      'heading',
    ];
    node.children.push({
      type: 'element',
      tagName: 'a',
      properties: {
        className: ['heading-anchor'],
        href: `#${id}`,
        ariaLabel: `Permalink to “${text(node)}”`,
      },
      children: [],
    });
    return;
  }

  if (Array.isArray(node.children)) {
    for (const child of node.children) walk(child, slugger);
  }
}

function toArray(value) {
  if (!value) return [];
  return Array.isArray(value) ? value : [value];
}

function text(node) {
  if (node.type === 'text') return node.value;
  if (!Array.isArray(node.children)) return '';
  return node.children.map(text).join('');
}
