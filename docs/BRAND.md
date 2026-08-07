# smartly — brand guideline

Internal reference. This content was deliberately removed from the public
website; it governs how smartly presents itself in the CLI, README, site,
and anything else user-facing. The fully designed version of this page
lives in git history (`site/src/pages/brand.astro`, removed in the same
commit that added this file) if it's ever needed again.

## Brand idea

**You know what you want to happen. Smartly knows the incantation.**

Everything in the product is one exchange: a sentence goes in, a command
comes out, and the command is right. The brand's job is to make that
exchange feel unremarkable — the way a very good colleague answering a
question is unremarkable — while being quietly precise about the moments
that carry consequences.

### Personality

| Trait | In practice |
|---|---|
| Calm | Nothing is urgent. Nothing is announced. |
| Capable | It has done this before, many times. |
| Precise | One command. The right flags. No hedging. |
| Fast | A sentence, then a result. No ceremony in between. |
| Lightly playful | One small wink, well placed. |
| Trustworthy around consequences | When something will actually run, it says so plainly. |

### What it is not

Not a magical assistant, not a copilot, not a teammate. Smartly does not
have a personality of its own in the interface — it has good manners.
Avoid anthropomorphic language, hype, and any suggestion that the model is
infallible.

## Taglines

- **Primary: "Tell your shell what you mean."** — the headline wherever
  Smartly is introduced. Sentence case, full stop included. Do not shorten.
- **Supporting: "You know what. Smartly knows how."** — sits under the
  primary, never instead of it. This is the one permitted wink; do not add
  a second one nearby. Marketing surfaces only.

## Logo

The wordmark plus the cyan terminal chevron is the primary mark
(`assets/smartly-logo-light.svg`, also at `site/public/brand/`). Use the
supplied SVG. Do not redraw, re-typeset, distort, or add effects.

- **On white**: the default.
- **On ink or photographic backgrounds**: keep the logo on its own white
  plate. A reversed lockup does not exist yet — knocking the wordmark out
  to white would be a re-typesetting of the mark. (Flag for a future dark
  lockup if one gets drawn.)

**Assets**: `smartly-logo-light.svg` (760×240, primary lockup),
`smartly-mark.svg` (the cyan `>_` alone, transparent), `favicon.svg`
(mark on ink plate, 512×512). The mark and favicon are the same two cyan
paths as the lockup — identical coordinates, stroke width and caps —
cropped/scaled, never redrawn.

**Clear space**: equal to the height of the chevron on every side (106
units in the SVG, ≈44% of asset height). Nothing sits inside that margin.

**Minimum sizes**: primary lockup 96px wide on screen (≈30px tall), 28mm
in print; mark alone 16px — below that, use the ink-plate favicon.

**Do**: use the supplied SVG above minimum size; scale uniformly; give it
full clear space; place on white, warm paper, or its plate over ink; use
the mark alone when the name is already nearby.

**Don't**: re-typeset the wordmark; stretch/condense/rotate/skew;
recolour chevron or wordmark; add shadows/glows/gradients/outlines; place
on busy imagery or inside a rounded pill; lock up with another logo
without clear space between.

## Text logo

Where an image cannot go, the canonical text form is:

```
smartly >_
```

- Always lowercase. **Smartly** is the brand in prose; `smartly` is the
  command and the logo.
- One space between the word and the `>_`.
- Set in Geist Mono where the medium allows.
- In color contexts: only the `>_` is electric cyan; the word stays in the
  default foreground.
- Never `>` alone, never `>>`, never a blinking cursor.

## Colour

| Name | Hex | Token | Role |
|---|---|---|---|
| Ink | `#151716` | `--ink` | Body text, dark surfaces, primary buttons. |
| White | `#FFFFFF` | `--white` | The primary surface. |
| Warm paper | `#F4F1E8` | `--paper` | Secondary surface: code blocks, bands, panels. |
| Electric cyan | `#00DDF5` | `--cyan` | Terminal symbols, active nav, focus halo, small delights. |
| Deep cyan | `#007F91` | `--cyan-deep` | Link underlines and cyan that has to survive on white. |
| Warning amber | `#FFB547` | `--amber` | The rule beside anything that will actually run. |
| Error red | `#F05D5E` | `--red` | Failure states only. Never decoration. |

Rules:

- **Electric cyan is for small, functional moments** — the `→` before a
  generated command, an active nav indicator, a focus halo. Not for large
  text on white (1.7:1, unreadable).
- **Deep cyan is the readable cyan** — wherever cyan must carry meaning as
  text or a control edge on a light surface.
- **Amber marks consequence, not mood.** Never a full amber panel, never
  general emphasis.
- **Red is failure only.** No decorative use.
- **Ink surfaces are rationed** — one dark moment per page at most, where
  the terminal genuinely belongs.
- Functional text and controls must clear 4.5:1. Two derived values exist
  for that and are documented, not improvised: `#006473` (cyan text on
  warm paper) and `#8A5200` (amber-family text).

## Typography

- **Instrument Sans** — interface, editorial, headlines. 400 body, 500
  controls, 600 headings. Tracking tightens as size grows (display sizes
  −0.035em).
- **Geist Mono** — code, terminal, configuration, labels, metadata. 400
  code, 500 labels. Eyebrows/small labels are the only uppercase (11px,
  0.14em tracking) — the only place Smartly shouts.
- Self-host both faces; no third-party font requests, ever.
- Anything a person would type or read in a terminal is Geist Mono;
  everything read as prose is Instrument Sans.
- Sentence case everywhere. No title case, no all-caps headlines.

## Terminal symbols

Documentation and marketing conventions — not output the CLI prints (the
CLI's own vocabulary lives in `internal/brand`):

| Glyph | Name | Meaning | Tone |
|---|---|---|---|
| `›` | Request | What a person typed, in plain English. | muted |
| `→` | Generated command | What smartly produced from that sentence. | cyan |
| `$` | Shell prompt | A command the reader types themselves. | cyan |
| `!` | Consequence | Something runs, is sent, or is written to disk. | amber |
| `·` | Nothing ran | Dry-run output, or a declined confirmation. | muted |

Use one pair per example: `›` for the request, `→` for the command. Don't
stack three or four glyphs on one illustration — the transformation is the
point, and it only has two sides.

## Voice

1. **Say less.** If a sentence can go, it goes.
2. **Prefer plain language.** No "leverage", no "seamless", no "powered by".
3. **Be confident without pretending the model is infallible.** State what
   happens. Never claim accuracy or safety the product does not have.
4. **Put humour around the workflow, never around risk.** Flag archaeology
   is funny. A destructive command is not.
5. **One small wink at a time.** A page gets one.
6. **Avoid hype and anthropomorphic AI language.** No copilots, no agents
   on your team, no unleashing anything.

### Copy examples

| Good | Bad | Why |
|---|---|---|
| Here's the command. | Your genius AI copilot has completed the mission! | State the result. The reader can see it is impressive. |
| Nothing ran. | No worries — nothing to see here! 🎉 | Two words, unambiguous. Emoji do not add certainty. |
| Commands without the ceremony. | Unleash the revolutionary power of AI! | Describe the product, not the category. |
| Describe the result. Skip the flag archaeology. | Let our intelligent agent handle the heavy lifting for you. | A concrete pain, named in the reader's own words. |

### Naming

- **Smartly** — the product, in prose and headlines.
- **`smartly`** — the binary, the command, the text logo. Always
  lowercase, always monospaced.
- Providers are written as their config values: `anthropic`, `openai`,
  `claude-cli`, `codex-cli`.

## In use

**README opening** (consequence stated in the opening, not buried — that
placement is part of the brand):

```
# smartly >_

Tell your shell what you mean.

    › remove all worktrees except main
    → git worktree remove /Users/you/project-fix

smartly turns one plain English sentence into one executable shell
command, and — by default — runs it immediately.

**Auto-run is the default, including for destructive commands.**
There is no confirmation prompt unless you configure one.
```

**Documentation callout** (title states the consequence, body states the
mechanism, last line gives the reader something to do — no apology, no
reassurance):

```
! This runs the command. Immediately.

  execution.mode defaults to auto: smartly generates one shell
  command and runs it with no confirmation prompt, including when
  the command is destructive. Add --dry-run until you have a feel
  for what it produces.
```
