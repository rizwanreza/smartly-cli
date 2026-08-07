/**
 * Documentation information architecture.
 *
 * The order here drives the sidebar, the previous/next controls and the search
 * index, so it is the single place to change when a page is added.
 */

export type NavItem = {
  /** Slug of the entry in the `docs` content collection. */
  slug: string;
  label: string;
};

export type NavGroup = {
  label: string;
  items: NavItem[];
};

export const docsNav: NavGroup[] = [
  {
    label: 'Start here',
    items: [
      { slug: 'getting-started', label: 'Getting started' },
      { slug: 'usage', label: 'Usage' },
      { slug: 'shell-integration', label: 'Shell integration' },
    ],
  },
  {
    label: 'Configure',
    items: [
      { slug: 'configuration', label: 'Configuration' },
      { slug: 'providers', label: 'Providers' },
      { slug: 'context', label: 'Context' },
    ],
  },
  {
    label: 'Run',
    items: [
      { slug: 'execution-and-safety', label: 'Execution and safety' },
      { slug: 'command-reference', label: 'Command reference' },
    ],
  },
];

/** Flat, ordered list used for previous/next navigation. */
export const docsOrder: NavItem[] = docsNav.flatMap((group) => group.items);

export function docHref(slug: string): string {
  return `/docs/${slug}/`;
}

export function neighbours(slug: string): {
  prev: NavItem | null;
  next: NavItem | null;
} {
  const i = docsOrder.findIndex((item) => item.slug === slug);
  if (i === -1) return { prev: null, next: null };
  return {
    prev: i > 0 ? docsOrder[i - 1]! : null,
    next: i < docsOrder.length - 1 ? docsOrder[i + 1]! : null,
  };
}
