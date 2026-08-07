/**
 * Wraps every Markdown table in a horizontally scrollable container.
 *
 * Making the table itself `display: block; overflow-x: auto` is the usual
 * shortcut, but it detaches the table box from its rows, so the border and
 * header fill stop short of the container. A real wrapper keeps the table a
 * table and lets the wrapper do the scrolling.
 */

export function rehypeWrapTables() {
  return function transformer(tree) {
    walk(tree);
  };

  function walk(node) {
    if (!node || !Array.isArray(node.children)) return;
    for (let i = 0; i < node.children.length; i++) {
      const child = node.children[i];
      if (child?.type === 'element' && child.tagName === 'table') {
        node.children[i] = {
          type: 'element',
          tagName: 'div',
          properties: { className: ['table-scroll'] },
          children: [child],
        };
        continue;
      }
      walk(child);
    }
  }
}
