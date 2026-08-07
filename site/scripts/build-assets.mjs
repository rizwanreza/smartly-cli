#!/usr/bin/env node
/**
 * Generates the raster brand assets that a static host needs but a browser
 * cannot derive from SVG: the Open Graph card, the Apple touch icon and the
 * legacy .ico favicon.
 *
 * Outputs are committed, so this only needs re-running when the source SVGs or
 * the OG copy change:
 *
 *   npm run build:assets
 *
 * The two cyan paths below are byte-identical to the ones in
 * assets/smartly-logo-light.svg — every raster mark stays exact geometry.
 */

import { writeFile, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Resvg } from '@resvg/resvg-js';

const root = fileURLToPath(new URL('..', import.meta.url));
const publicDir = join(root, 'public');
const fontDir = join(root, 'src/assets/fonts-ttf');

// resvg renders the default instance of a variable font and ignores
// font-weight, so the headline face is a static wght=600 instance of Instrument
// Sans, produced once with:
//   fonttools varLib.instancer "InstrumentSans[wdth,wght].ttf" wght=600 wdth=100
//   pyftsubset InstrumentSans-SemiBold.ttf --unicodes="U+0020-007E,..."
// All three files are latin-subset and used only by this script — the site
// itself ships woff2 from src/assets/fonts.
const fontFiles = [
  join(fontDir, 'InstrumentSans-SemiBold.ttf'),
  join(fontDir, 'GeistMono-Regular.ttf'),
  join(fontDir, 'GeistMono-Medium.ttf'),
];

const INK = '#151716';
const PAPER = '#F4F1E8';
const CYAN = '#00DDF5';
const MUTED = '#5F6461';

/* ------------------------------------------------------------- og card --- */

const logo = await readFile(join(publicDir, 'brand/smartly-logo-light.svg'), 'utf8');
// Reuse the wordmark + chevron group from the shipped asset verbatim.
const logoInner = logo.slice(logo.indexOf('<g\n     transform="translate(52)"'), logo.lastIndexOf('</svg>'));

const og = `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <rect width="1200" height="630" fill="${PAPER}"/>
  <rect x="0" y="0" width="1200" height="6" fill="${CYAN}"/>

  <g transform="translate(80, 52) scale(0.42)">${logoInner}</g>

  <text x="80" y="288" font-family="Instrument Sans" font-size="82" letter-spacing="-2.9" fill="${INK}">Tell your shell what you mean.</text>
  <text x="80" y="344" font-family="Geist Mono" font-weight="500" font-size="25" fill="#006473">You know what. Smartly knows how.</text>

  <line x1="80" y1="418" x2="1120" y2="418" stroke="#DCD6C5" stroke-width="1"/>

  <text x="80" y="474" font-family="Geist Mono" font-weight="400" font-size="25" fill="${MUTED}">›  find all occurrences of git and replace with svn</text>
  <text x="80" y="524" font-family="Geist Mono" font-weight="500" font-size="28" fill="${INK}"><tspan fill="${CYAN}">→</tspan>  find . -type f -exec sed -i '' 's/git/svn/g' {} +</text>

  <text x="80" y="580" font-family="Geist Mono" font-weight="400" font-size="19" fill="${MUTED}">One sentence in. One shell command out. Auto-run is the default.</text>
</svg>`;

await render(og, 1200, join(publicDir, 'og.png'));

/* ---------------------------------------------------------------- icons --- */

const favicon = await readFile(join(publicDir, 'favicon.svg'), 'utf8');

await render(favicon, 180, join(publicDir, 'apple-touch-icon.png'));

const ico32 = await rasterize(favicon, 32);
await writeFile(join(publicDir, 'favicon.ico'), wrapIco(ico32, 32));

console.log('build-assets: wrote og.png, apple-touch-icon.png, favicon.ico');

/* ---------------------------------------------------------------- utils --- */

async function rasterize(svg, width) {
  const resvg = new Resvg(svg, {
    fitTo: { mode: 'width', value: width },
    font: { fontFiles, loadSystemFonts: false, defaultFontFamily: 'Instrument Sans' },
  });
  return resvg.render().asPng();
}

async function render(svg, width, out) {
  await writeFile(out, await rasterize(svg, width));
}

/**
 * Minimal ICO container around a single PNG frame. The ICO format has allowed
 * PNG-compressed entries since Vista, and every browser that still asks for
 * favicon.ico understands them.
 */
function wrapIco(png, size) {
  const header = Buffer.alloc(6);
  header.writeUInt16LE(0, 0); // reserved
  header.writeUInt16LE(1, 2); // type: icon
  header.writeUInt16LE(1, 4); // one image

  const entry = Buffer.alloc(16);
  entry.writeUInt8(size === 256 ? 0 : size, 0); // width
  entry.writeUInt8(size === 256 ? 0 : size, 1); // height
  entry.writeUInt8(0, 2); // palette
  entry.writeUInt8(0, 3); // reserved
  entry.writeUInt16LE(1, 4); // colour planes
  entry.writeUInt16LE(32, 6); // bits per pixel
  entry.writeUInt32LE(png.length, 8);
  entry.writeUInt32LE(header.length + entry.length, 12);

  return Buffer.concat([header, entry, png]);
}
