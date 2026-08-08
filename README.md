<p align="center">
  <img src="./assets/smartly-logo-light.svg" alt="smartly >_" width="380">
</p>

<p align="center">
  <strong>Tell your shell what you mean.</strong>
</p>

<p align="center">
  <a href="https://github.com/rizwanreza/smartly-cli/releases"><img src="https://img.shields.io/github/v/release/rizwanreza/smartly-cli?style=flat-square" alt="Latest release"></a>
  <a href="https://github.com/rizwanreza/smartly-cli/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/rizwanreza/smartly-cli/ci.yml?branch=main&style=flat-square" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-007F91?style=flat-square" alt="MIT license"></a>
</p>

<p align="center">
  <a href="https://smartlycli.com/">smartlycli.com</a> ·
  <a href="https://smartlycli.com/docs/getting-started/">Getting started</a> ·
  <a href="https://smartlycli.com/docs/execution-and-safety/">Execution and safety</a>
</p>

---

`smartly` turns a plain English sentence into a single shell command and, by
default, runs it immediately.

```
$ smartly remove all worktrees except main
→ git worktree remove /Users/you/project-fix
```

**Auto-run is the default, including for destructive commands.** There is no
confirmation prompt unless you configure one. See [Execution
mode](#execution-mode) if you want a safety net — `confirm-destructive` asks
on anything it doesn't recognize as safe, destructive or unrecognized alike.

## Install

```
brew install rizwanreza/tap/smartly

# or, if you'd rather build it yourself
go install github.com/rizwanreza/smartly-cli/cmd/smartly@latest
```

The macOS binaries are signed with an Apple Developer ID and notarized, so
Gatekeeper lets them run with no extra flags; they're also on the [releases
page](https://github.com/rizwanreza/smartly-cli/releases).

Out of the box smartly uses `provider: anthropic`, so it needs only
`ANTHROPIC_API_KEY` set in your environment. No config file is required.

## `smartly onboard`

To be walked through the settings, run `smartly onboard`. It asks which
model to use, how careful you want smartly to be, and how much of your
environment it gets to see, then shows you the whole config before writing
anything. **It never asks for an API key** — it checks whether the
environment variable is set and prints the `export …` line if it isn't; no
key value is typed into it, shown by it, or written to `config.yaml` by it.
It never edits your rc file, backs up an existing config first, writes
nothing until you confirm, and fails closed with no terminal. You don't need
it: smartly runs on defaults with `ANTHROPIC_API_KEY` set.

## Usage

The whole interface is `smartly <your request in plain English>`.

```
$ smartly show hidden files sorted by size
→ ls -lahS

$ smartly what changed in this repo in the last week
→ git log --oneline --since='1 week ago'

$ smartly kill whatever is listening on port 3000
→ kill $(lsof -ti :3000)

$ smartly delete all my branches that are already merged into main
→ git branch --merged main | grep -vE '^\*| main$' | xargs git branch -d

$ smartly replace api.example.com with api.internal in every yaml file
→ find . -name '*.y*ml' -exec sed -i '' 's/api\.example\.com/api.internal/g' {} +
```

smartly sends your sentence — plus, optionally, some context about your
directory and git state — to an LLM, validates the response is a single
command line, and runs it. It's for people who know what they want to do but
don't want to remember exact flags.

A few things worth knowing before you rely on it:

- It's aware of your current directory and git state (branch, status,
  worktrees) by default, so requests that reference "it" or "all worktrees
  except main" resolve against what's actually there. See [Context
  levels](#context-levels).
- It accounts for GNU (Linux) vs BSD (macOS) userland differences — `sed -i
  ''` on macOS vs `sed -i` on Linux, and so on.
- To see the command before it runs, use `--confirm` or set
  `execution.mode` — see [Execution mode](#execution-mode).

## Shell integration

Some generated commands change your shell's state — `cd` into a directory,
`export` a variable — which a subprocess can't do for you. Source smartly's
shell function to make that work:

```bash
# bash: add to ~/.bashrc
eval "$(smartly init bash)"

# zsh: add to ~/.zshrc
eval "$(smartly init zsh)"
```

Without it, `smartly` still works for anything that doesn't mutate shell
state (most commands), but a generated `cd` or `export` only affects the
subprocess it ran in.

## Output

smartly uses four symbols, so its own output is always distinguishable from
the output of the command it ran:

```
→ generated command
✓ successful operation
! warning or confirmation
× smartly error
```

Everything smartly says goes to **stderr** — except the confirmation prompt
and `onboard`, which write to `/dev/tty` directly, and root `--help`, prose
for a human. stdout carries only results something else consumes: the
command from `--print-only` or `--dry-run`, the script from `smartly init`,
the settings from `config show`. Piping and command substitution stay exact.

Color appears only when the stream being written to is a terminal — stderr,
for status output — with `NO_COLOR` unset and `TERM` not `dumb`. It never
carries meaning alone: the symbols above distinguish the lines, and they
survive with color stripped.

## Configuration

Config lives at `$XDG_CONFIG_HOME/smartly/config.yaml`, or
`~/.config/smartly/config.yaml` if `XDG_CONFIG_HOME` isn't set. None of it is
required. `smartly config init` writes a default one; `config show` prints
the resolved settings with secrets redacted; `config path`, where it lives.

```yaml
provider: anthropic     # anthropic | openai | claude-cli | codex-cli
execution:
  mode: auto            # auto | confirm | confirm-destructive
context: light          # none | light | full
providers:
  anthropic:
    model: claude-opus-5
```

That's a fraction of it — the full schema, every field and its default, is
at [Configuration](https://smartlycli.com/docs/configuration/). API keys
resolve in order: the env var named by `api_key_env`, then the provider's
default env var (`ANTHROPIC_API_KEY` / `OPENAI_API_KEY`), then the config
file's `api_key` fallback.

### Execution mode

- `auto` (default): generate and run immediately, no prompt.
- `confirm`: show the command and ask before running it, every time:

  ```
  → git worktree remove /Users/you/project-fix

  ! Run this command? [y/N]
  ```

- `confirm-destructive`: asks on anything it doesn't recognize as safe —
  destructive or unrecognized alike. A safe command runs straight through;
  anything else stops and explains itself:

  ```
  → rm -rf ./build

  ! rm deletes files
    Run it? [y/N]
  ```

Confirmation reads from `/dev/tty` directly, so it works even when stdin is
otherwise in use. With no controlling terminal (CI, cron, a fully
non-interactive pipe) it **fails closed** — it will not run the command —
rather than hang or silently proceed. Use `-y`/`--yes` there.

Per invocation: `--confirm` forces the prompt, `-y`/`--yes` forces auto-run
(mutually exclusive), and `--dry-run` prints the command, plus what would
have happened to it, without asking or running it. Full behavior at
[Execution mode](https://smartlycli.com/docs/execution-and-safety/#execution-mode).

#### What counts as destructive

`confirm-destructive` uses a local static classifier. It reads the generated
command string and nothing else — no filesystem checks, no network, no
second trip to the LLM — and returns one of three verdicts: **safe**
(recognized, read-only or purely additive), **destructive** (recognized and
known to mutate something), or **unknown** (not recognized at all). Safe
runs; the other two ask.

Unknown asking is the deliberate part: a seatbelt that silently passes what
it doesn't recognize is worse than no seatbelt. The cost is real — it will
prompt for commands that are perfectly harmless, just unrecognized.

**This is a best-effort seatbelt, not a sandbox.** It reads text, not
intent, and false negatives are possible by construction: quoting tricks, an
allowlisted tool coaxed into writing (`awk '{print > "f"}'`), or a binary
that isn't in the tables can all slip through. If you want to see every
command before it runs, use `confirm`, not `confirm-destructive`.

Full verdict tables at [What counts as
destructive](https://smartlycli.com/docs/execution-and-safety/#what-counts-as-destructive).

### Providers

| Provider | Auth | Worth knowing |
|---|---|---|
| `anthropic` | `ANTHROPIC_API_KEY` | The default. Nothing else to set up. |
| `openai` | `OPENAI_API_KEY` | No default model — set `providers.openai.model` yourself. |
| `claude-cli` | your `claude` login | Shells out to the CLI you're already paying for. |
| `codex-cli` | your `codex` login | Same, via `codex`. |

The `openai` provider is not limited to OpenAI: it speaks the Chat
Completions API, so `base_url` plus `api_key_env` points it at Fireworks,
Together, Groq, OpenRouter, Azure OpenAI, or a local vLLM / Ollama server.
A `base_url` sends your prompt — and, with `context: light`, your directory
listing and git status — to whatever you point it at.

The two CLI providers each differ in one visible way: `claude-cli` runs with
no tool access at all, so it can only answer in text; `codex-cli` is
sandboxed read-only but still agentic while composing, so its reply
occasionally narrates a failed self-attempted command. Setup and the
tradeoffs in full at [Providers](https://smartlycli.com/docs/providers/).

### Context levels

- `none`: only your sentence is sent.
- `light` (default): adds a capped directory listing plus git
  branch/status/worktree info, so "all worktrees except main" resolves.
- `full`: `light` plus a tail of your recent shell history (`$HISTFILE`, or
  `~/.zsh_history` / `~/.bash_history` by shell).

**`full` is never the default and you should turn it on deliberately.** Your
shell history can contain secrets typed inline (tokens, passwords pasted
into a command), and enabling `full` context sends that history to a
third-party LLM API on every request.

### Logging

Every generate-and-run invocation is appended to `log.path` as JSONL: a
`request` record, then a `completion` record correlated by `request_id`. The
log is strictly append-only and created `0600`. Each request carries a `risk`
field (`safe` / `destructive` / `unknown`) with the classifier's verdict,
recorded whatever your execution mode is, so `auto` users can still audit.

**The log stores your raw sentences and generated commands verbatim**, which
may include anything sensitive you typed — treat it like shell history.
Record shape at [Logging](https://smartlycli.com/docs/execution-and-safety/#logging).

## Limitations

- Generates a single shell command line per invocation (pipes, `&&`,
  redirects within that line are fine; multi-step scripts are not).
- Targets bash/zsh on Linux and macOS. Windows/PowerShell isn't supported
  yet.
- `claude-cli`/`codex-cli` need that CLI installed and separately logged in
  (`claude login` / `codex login`) — smartly does not manage that login
  state — and their error classification is best-effort.

## Documentation

Run `smartly --help` for the flags. The reference manual is at
[smartlycli.com](https://smartlycli.com/):
[Getting started](https://smartlycli.com/docs/getting-started/) ·
[Usage](https://smartlycli.com/docs/usage/) ·
[Configuration](https://smartlycli.com/docs/configuration/) ·
[Providers](https://smartlycli.com/docs/providers/) ·
[Execution and safety](https://smartlycli.com/docs/execution-and-safety/) ·
[Context](https://smartlycli.com/docs/context/) ·
[Shell integration](https://smartlycli.com/docs/shell-integration/) ·
[Command reference](https://smartlycli.com/docs/command-reference/)

## License

MIT. See [LICENSE](./LICENSE).

## Contributing

Issues and pull requests are welcome. Run `go vet ./... && go test ./...`
before opening one, and keep user-facing copy — CLI output, help text,
errors, this README — consistent with [docs/BRAND.md](./docs/BRAND.md).
