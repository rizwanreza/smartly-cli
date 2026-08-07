import type { APIRoute } from 'astro';
import { getCollection } from 'astro:content';
import GithubSlugger from 'github-slugger';

import { docsOrder, docHref } from '../lib/nav';
import { withBase } from '../lib/url';

/**
 * Static search index, one record per heading section. Built at compile time
 * and fetched lazily the first time the search dialog opens, so it costs
 * nothing on initial page load.
 */

type Record = {
  url: string;
  page: string;
  heading: string;
  text: string;
};

export const GET: APIRoute = async () => {
  const entries = await getCollection('docs');
  const byId = new Map(entries.map((entry) => [entry.id, entry]));
  const records: Record[] = [];

  for (const item of docsOrder) {
    const entry = byId.get(item.slug);
    if (!entry) continue;

    const base = withBase(docHref(item.slug));
    const slugger = new GithubSlugger();

    // The page itself, so a query matching only the lede still finds it.
    records.push({
      url: base,
      page: item.label,
      heading: entry.data.title,
      text: `${entry.data.lede} ${entry.data.description}`,
    });

    for (const section of splitSections(entry.body ?? '')) {
      records.push({
        url: `${base}#${slugger.slug(section.heading)}`,
        page: item.label,
        heading: section.heading,
        text: section.text,
      });
    }
  }

  // The brand guideline is a hand-built page, so its sections are listed here
  // rather than derived from a collection.
  const brand = withBase('/brand/');
  for (const section of BRAND_SECTIONS) {
    records.push({
      url: `${brand}#${section.id}`,
      page: 'Brand',
      heading: section.heading,
      text: section.text,
    });
  }

  return new Response(JSON.stringify(records), {
    headers: { 'content-type': 'application/json; charset=utf-8' },
  });
};

const BRAND_SECTIONS = [
  {
    id: 'idea',
    heading: 'Brand idea',
    text: 'Quiet magic with a slightly sly voice. Calm, capable, precise, fast, lightly playful, trustworthy around consequential actions. Personality.',
  },
  {
    id: 'taglines',
    heading: 'Taglines',
    text: 'Tell your shell what you mean. You know what. Smartly knows how. Primary and supporting taglines.',
  },
  {
    id: 'logo',
    heading: 'Logo',
    text: 'Logo previews on white and ink, downloads, clear space, minimum sizes, correct and incorrect usage.',
  },
  {
    id: 'text-logo',
    heading: 'Text logo',
    text: 'The canonical text logo smartly >_ for READMEs, terminal banners and plain-text contexts.',
  },
  {
    id: 'colour',
    heading: 'Colour',
    text: 'Ink, white, warm paper, electric cyan, deep cyan, warning amber, error red. Hex values and usage rules.',
  },
  {
    id: 'typography',
    heading: 'Typography',
    text: 'Instrument Sans for interface and editorial text. Geist Mono for code, terminal content, configuration, labels and metadata. Self-hosted.',
  },
  {
    id: 'symbols',
    heading: 'Terminal symbols',
    text: 'Request, generated command, shell prompt, consequence, nothing ran. Documentation conventions and their meanings.',
  },
  {
    id: 'voice',
    heading: 'Voice',
    text: 'Say less. Prefer plain language. Be confident without pretending the model is infallible. Humour around the workflow, never around risk. Good and bad copy examples.',
  },
  {
    id: 'in-use',
    heading: 'In use',
    text: 'README opening and documentation callout examples showing the voice and symbols together.',
  },
];

/** Split raw MDX into `## `/`### ` sections of plain, searchable text. */
function splitSections(body: string): { heading: string; text: string }[] {
  const sections: { heading: string; text: string }[] = [];
  let current: { heading: string; text: string[] } | null = null;
  let inFence = false;

  for (const line of body.split('\n')) {
    if (/^\s*```/.test(line)) {
      inFence = !inFence;
      continue;
    }
    if (inFence) continue;

    const heading = line.match(/^(#{2,3})\s+(.*)$/);
    if (heading) {
      if (current) {
        sections.push({ heading: current.heading, text: clean(current.text) });
      }
      current = { heading: stripInline(heading[2]!), text: [] };
      continue;
    }
    if (current) current.text.push(line);
  }

  if (current) sections.push({ heading: current.heading, text: clean(current.text) });
  return sections.filter((section) => section.heading.length > 0);
}

function clean(lines: string[]): string {
  return stripInline(
    lines
      .join(' ')
      .replace(/<[^>]+>/g, ' ') // JSX / HTML tags
      .replace(/^\s*import .*$/gm, ' '),
  ).slice(0, 600);
}

function stripInline(value: string): string {
  return value
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1') // links and images
    .replace(/[`*_>|]/g, '')
    .replace(/\{[^}]*\}/g, ' ') // JSX expressions
    .replace(/\s+/g, ' ')
    .trim();
}
