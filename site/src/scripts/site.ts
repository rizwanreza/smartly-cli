/**
 * Site behaviour. No framework — five small, independent modules:
 * copy buttons, platform tabs, the mobile drawer, documentation search and
 * active-heading tracking. Everything degrades to a usable static page if this
 * file fails to load.
 */

/* ------------------------------------------------------------------ copy */

const COPY_IDLE = 'Copy';
const COPY_DONE = 'Copied';

function initCopyButtons(): void {
  const blocks = document.querySelectorAll<HTMLPreElement>(
    '[data-copyable] pre, .code-block > pre',
  );

  blocks.forEach((pre) => {
    let wrapper = pre.parentElement;
    if (!wrapper?.classList.contains('code-block')) {
      wrapper = document.createElement('div');
      wrapper.className = 'code-block';
      pre.replaceWith(wrapper);
      wrapper.appendChild(pre);
    }
    if (wrapper.querySelector('.copy-btn')) return;

    const label = pre.dataset.copyLabel ?? 'code';
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'copy-btn';
    button.textContent = COPY_IDLE;
    button.setAttribute('aria-label', `Copy ${label} to clipboard`);

    button.addEventListener('click', async () => {
      const text = pre.innerText.replace(/\n$/, '');
      const ok = await writeClipboard(text);
      button.textContent = ok ? COPY_DONE : 'Press ⌘C';
      button.dataset.copied = 'true';
      window.setTimeout(() => {
        button.textContent = COPY_IDLE;
        delete button.dataset.copied;
      }, 1800);
    });

    wrapper.appendChild(button);
  });
}

async function writeClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // Non-secure contexts and older browsers: fall back to a hidden textarea.
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    let ok = false;
    try {
      ok = document.execCommand('copy');
    } catch {
      ok = false;
    }
    ta.remove();
    return ok;
  }
}

/**
 * Generic "copy this literal value" control — used by the brand page's colour
 * tokens and text logo. The element carries the value; the element with
 * [data-copy-feedback] inside it (or the element itself) shows the result.
 */
function initValueCopy(): void {
  document
    .querySelectorAll<HTMLElement>('[data-copy-value]')
    .forEach((button) => {
      const feedback =
        button.querySelector<HTMLElement>('[data-copy-feedback]') ?? button;
      const original = feedback.textContent ?? '';
      button.addEventListener('click', async () => {
        const ok = await writeClipboard(button.dataset.copyValue ?? '');
        feedback.textContent = ok ? 'Copied' : 'Press ⌘C';
        button.dataset.copied = 'true';
        window.setTimeout(() => {
          feedback.textContent = original;
          delete button.dataset.copied;
        }, 1600);
      });
    });
}

/* ------------------------------------------------------------------ tabs */

const TAB_STORAGE_KEY = 'smartly:platform';

function initTabs(): void {
  const groups = document.querySelectorAll<HTMLElement>('[data-tabs]');
  if (!groups.length) return;

  const stored = safeRead(TAB_STORAGE_KEY);

  groups.forEach((group) => {
    const tabs = Array.from(
      group.querySelectorAll<HTMLButtonElement>('[role="tab"]'),
    );

    tabs.forEach((tab, index) => {
      tab.addEventListener('click', () => select(group, tab.dataset.key!, true));
      tab.addEventListener('keydown', (event) => {
        const delta =
          event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
        if (!delta) return;
        event.preventDefault();
        const next = tabs[(index + delta + tabs.length) % tabs.length]!;
        next.focus();
        select(group, next.dataset.key!, true);
      });
    });

    if (stored && tabs.some((tab) => tab.dataset.key === stored)) {
      select(group, stored, false);
    }
  });

  function select(origin: HTMLElement, key: string, persist: boolean): void {
    // Platform choice is a whole-page preference: switching one group switches
    // every group so a reader never has to re-pick per code block.
    const shared = origin.dataset.tabs === 'platform';
    const targets = shared
      ? document.querySelectorAll<HTMLElement>('[data-tabs="platform"]')
      : [origin];

    targets.forEach((group) => {
      group
        .querySelectorAll<HTMLButtonElement>('[role="tab"]')
        .forEach((tab) => {
          const on = tab.dataset.key === key;
          tab.setAttribute('aria-selected', String(on));
          tab.tabIndex = on ? 0 : -1;
        });
      group
        .querySelectorAll<HTMLElement>('[role="tabpanel"]')
        .forEach((panel) => {
          panel.hidden = panel.dataset.key !== key;
        });
    });

    if (persist && shared) safeWrite(TAB_STORAGE_KEY, key);
  }
}

function safeRead(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function safeWrite(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    /* private mode — preference is simply not remembered */
  }
}

/* ------------------------------------------------- modal focus handling */

let lastFocused: HTMLElement | null = null;

const FOCUSABLE =
  'a[href], button:not([disabled]), input, select, textarea, [tabindex]:not([tabindex="-1"])';

function openModal(root: HTMLElement, focusTarget?: HTMLElement | null): void {
  lastFocused = document.activeElement as HTMLElement | null;
  root.hidden = false;
  document.body.style.overflow = 'hidden';
  const target =
    focusTarget ?? root.querySelector<HTMLElement>(FOCUSABLE) ?? root;
  target.focus();
}

function closeModal(root: HTMLElement): void {
  root.hidden = true;
  document.body.style.overflow = '';
  lastFocused?.focus();
  lastFocused = null;
}

function trapFocus(root: HTMLElement, event: KeyboardEvent): void {
  if (event.key !== 'Tab') return;
  const nodes = Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (node) => node.offsetParent !== null || node === document.activeElement,
  );
  if (!nodes.length) return;
  const first = nodes[0]!;
  const last = nodes[nodes.length - 1]!;
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

/* ---------------------------------------------------------------- drawer */

function initDrawer(): void {
  const drawer = document.querySelector<HTMLElement>('[data-drawer]');
  const trigger = document.querySelector<HTMLButtonElement>('[data-drawer-open]');
  if (!drawer || !trigger) return;

  const open = () => {
    // No explicit target: the first focusable node inside the drawer is the
    // close button, which is where focus belongs on open.
    openModal(drawer);
    trigger.setAttribute('aria-expanded', 'true');
  };

  const close = () => {
    if (drawer.hidden) return;
    closeModal(drawer);
    trigger.setAttribute('aria-expanded', 'false');
  };

  trigger.addEventListener('click', open);
  drawer
    .querySelectorAll('[data-drawer-close]')
    .forEach((node) => node.addEventListener('click', close));

  drawer.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') close();
    trapFocus(drawer, event);
  });

  const wide = window.matchMedia('(min-width: 64rem)');
  wide.addEventListener('change', (event) => {
    if (event.matches) close();
  });
}

/* ---------------------------------------------------------------- search */

type SearchDoc = {
  url: string;
  page: string;
  heading: string;
  text: string;
};

function initSearch(): void {
  const root = document.querySelector<HTMLElement>('[data-search]');
  if (!root) return;

  const input = root.querySelector<HTMLInputElement>('[data-search-input]')!;
  const results = root.querySelector<HTMLUListElement>('[data-search-results]')!;
  const empty = root.querySelector<HTMLElement>('[data-search-empty]')!;
  const indexUrl = root.dataset.index!;

  let docs: SearchDoc[] | null = null;
  let loading: Promise<void> | null = null;
  let cursor = -1;

  const load = () => {
    if (docs || loading) return loading ?? Promise.resolve();
    loading = fetch(indexUrl)
      .then((res) => res.json())
      .then((data: SearchDoc[]) => {
        docs = data;
      })
      .catch(() => {
        docs = [];
        empty.textContent = 'Search index unavailable.';
      });
    return loading;
  };

  const open = () => {
    void load();
    openModal(root, input);
    input.select();
  };

  const close = () => {
    if (root.hidden) return;
    closeModal(root);
  };

  document
    .querySelectorAll('[data-search-open]')
    .forEach((node) => node.addEventListener('click', open));
  root
    .querySelectorAll('[data-search-close]')
    .forEach((node) => node.addEventListener('click', close));

  document.addEventListener('keydown', (event) => {
    const inField =
      document.activeElement instanceof HTMLElement &&
      /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement.tagName);
    const slash = event.key === '/' && !inField && root.hidden;
    const cmdK = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k';
    if (slash || cmdK) {
      event.preventDefault();
      open();
    }
  });

  root.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      close();
      return;
    }
    trapFocus(root, event);
  });

  input.addEventListener('keydown', (event) => {
    const items = Array.from(results.querySelectorAll('li'));
    if (!items.length) return;
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      cursor =
        (cursor + (event.key === 'ArrowDown' ? 1 : -1) + items.length) %
        items.length;
      items.forEach((li, i) =>
        li.setAttribute('aria-selected', String(i === cursor)),
      );
      items[cursor]!.scrollIntoView({ block: 'nearest' });
    } else if (event.key === 'Enter' && cursor >= 0) {
      event.preventDefault();
      items[cursor]!.querySelector('a')?.click();
    }
  });

  input.addEventListener('input', () => {
    void load().then(() => render(input.value.trim()));
  });

  function render(query: string): void {
    cursor = -1;
    results.innerHTML = '';
    input.setAttribute('aria-expanded', 'false');

    if (query.length < 2) {
      empty.hidden = false;
      empty.textContent = 'Type to search. Enter opens, arrows move.';
      return;
    }

    const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
    const matches = (docs ?? [])
      .map((doc) => ({ doc, score: score(doc, terms) }))
      .filter((entry) => entry.score > 0)
      .sort((a, b) => b.score - a.score)
      .slice(0, 8);

    if (!matches.length) {
      empty.hidden = false;
      empty.textContent = `Nothing matched “${query}”.`;
      return;
    }

    empty.hidden = true;
    input.setAttribute('aria-expanded', 'true');

    for (const { doc } of matches) {
      const li = document.createElement('li');
      li.setAttribute('role', 'option');
      li.setAttribute('aria-selected', 'false');

      const a = document.createElement('a');
      a.href = doc.url;

      const crumb = document.createElement('span');
      crumb.className = 'search__crumb';
      crumb.textContent = doc.page;

      const title = document.createElement('span');
      title.className = 'search__title';
      title.append(...highlight(doc.heading, terms));

      const snippet = document.createElement('span');
      snippet.className = 'search__snippet';
      snippet.append(...highlight(excerpt(doc.text, terms), terms));

      a.append(crumb, title, snippet);
      li.append(a);
      results.append(li);
    }
  }

  function score(doc: SearchDoc, terms: string[]): number {
    const heading = doc.heading.toLowerCase();
    const page = doc.page.toLowerCase();
    const text = doc.text.toLowerCase();
    let total = 0;
    for (const term of terms) {
      if (heading.includes(term)) total += 8;
      else if (page.includes(term)) total += 4;
      else if (text.includes(term)) total += 2;
      else return 0; // every term must appear somewhere
    }
    return total;
  }
}

function excerpt(text: string, terms: string[]): string {
  const lower = text.toLowerCase();
  const at = terms.map((t) => lower.indexOf(t)).find((i) => i >= 0) ?? 0;
  const start = Math.max(0, at - 30);
  return (start ? '…' : '') + text.slice(start, start + 130);
}

function highlight(text: string, terms: string[]): Node[] {
  const pattern = terms
    .map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    .filter(Boolean)
    .join('|');
  if (!pattern) return [document.createTextNode(text)];

  const nodes: Node[] = [];
  const re = new RegExp(`(${pattern})`, 'ig');
  let last = 0;
  let match: RegExpExecArray | null;
  while ((match = re.exec(text))) {
    if (match.index > last) {
      nodes.push(document.createTextNode(text.slice(last, match.index)));
    }
    const mark = document.createElement('mark');
    mark.textContent = match[0];
    nodes.push(mark);
    last = match.index + match[0].length;
  }
  if (last < text.length) nodes.push(document.createTextNode(text.slice(last)));
  return nodes;
}

/* -------------------------------------------------------- active heading */

function initTocHighlight(): void {
  const toc = document.querySelector<HTMLElement>('[data-toc]');
  if (!toc) return;

  const links = Array.from(toc.querySelectorAll<HTMLAnchorElement>('a[href^="#"]'));
  const headings = links
    .map((link) => document.getElementById(decodeURIComponent(link.hash.slice(1))))
    .filter((el): el is HTMLElement => Boolean(el));
  if (!headings.length) return;

  const setActive = (id: string) => {
    links.forEach((link) => {
      const on = decodeURIComponent(link.hash.slice(1)) === id;
      link.setAttribute('aria-current', on ? 'true' : 'false');
    });
  };

  const observer = new IntersectionObserver(
    (entries) => {
      const visible = entries
        .filter((entry) => entry.isIntersecting)
        .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
      if (visible[0]) setActive(visible[0].target.id);
    },
    { rootMargin: '-80px 0px -70% 0px', threshold: 0 },
  );

  headings.forEach((heading) => observer.observe(heading));
  setActive(headings[0]!.id);
}

/* ------------------------------------------------------------------ boot */

function boot(): void {
  initCopyButtons();
  initValueCopy();
  initTabs();
  initDrawer();
  initSearch();
  initTocHighlight();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', boot, { once: true });
} else {
  boot();
}
