# smartly

Turn a plain English sentence into an executable shell command.

```
$ smartly find all occurrences of git and replace with svn
$ git worktree remove /Users/you/project-fix
$ smartly tail logs from development.log
$ tail -f development.log
```

smartly sends your sentence (plus, optionally, a bit of context about your
current directory and git state) to an LLM, sanitizes the response into a
single shell command, and — by default — runs it immediately. It's meant
for people who know what they want to do but don't want to remember exact
flags, and it accounts for GNU (Linux) vs BSD (macOS) userland differences
when generating commands.

**Auto-run is the default, including for destructive commands.** There is
no confirmation prompt unless you configure one. See [Execution
mode](#execution-mode) below if you want a safety net.

## Install

```
go install github.com/rizwanreza/smartly-cli/cmd/smartly@latest
```

(A Homebrew tap is planned once this is stable enough to tag a release —
see `.goreleaser.yaml`.)

## Usage

```
smartly <your request in plain English>
```

smartly generates one shell command and, by default, runs it immediately:

```
$ smartly remove all worktrees except main
$ git worktree remove /Users/you/project-fix
$ smartly find all occurrences of git and replace with svn
$ find . -type f -exec sed -i '' 's/git/svn/g' {} +
```

A few things worth knowing before you rely on it:

- It defaults to `provider: anthropic`, so out of the box it just needs
  `ANTHROPIC_API_KEY` set in your environment — no config file required.
- It's aware of your current directory and git state (branch, status,
  worktrees) by default, so requests that reference "it" or "all
  worktrees except main" resolve against what's actually there. See
  [Context levels](#context-levels).
- It generates commands appropriate to your OS (`sed -i ''` on macOS/BSD
  vs `sed -i` on Linux/GNU, etc.).
- If you'd rather see the command before it runs, use `--confirm` or set
  `execution.mode: confirm` — see [Execution mode](#execution-mode).
- Only one shell command line is generated per invocation. Pipes, `&&`,
  and redirects within that line are fine; smartly won't produce
  multi-step scripts.

## Shell integration

Some generated commands need to change your shell's state — `cd` into a
directory, `export` a variable — which a plain subprocess can't do for you.
Source smartly's shell function to make that work:

```bash
# bash: add to ~/.bashrc
eval "$(smartly init bash)"

# zsh: add to ~/.zshrc
eval "$(smartly init zsh)"
```

Without this, `smartly` still works for anything that doesn't need to
mutate your shell's state (most commands), but a generated `cd` or `export`
will only affect the subprocess it ran in, not your interactive shell.

## Configuration

Config lives at `$XDG_CONFIG_HOME/smartly/config.yaml`, or
`~/.config/smartly/config.yaml` if `XDG_CONFIG_HOME` isn't set. None of it
is required — smartly runs on defaults as long as `ANTHROPIC_API_KEY` is
set in your environment.

```
smartly config init    # write a default config.yaml
smartly config show    # print the resolved config (secrets redacted)
smartly config path    # print the resolved config file path
```

Full schema:

```yaml
provider: anthropic          # anthropic | openai | claude-cli | codex-cli

execution:
  mode: auto                 # auto | confirm

context: light                # none | light | full

log:
  path: ~/.config/smartly/history.log

providers:
  anthropic:
    model: claude-opus-5
    api_key_env: ANTHROPIC_API_KEY
    api_key: ""              # fallback only, used if the env var is unset
    base_url: ""              # optional: self-hosted/proxy endpoint

  openai:
    model: ""                 # required if provider: openai — no default shipped
    api_key_env: OPENAI_API_KEY
    api_key: ""
    base_url: ""              # optional: Azure OpenAI, vLLM, LM Studio, etc.

  claude-cli:
    model: haiku               # required — see CLI-based authentication below
    binary: claude
    max_budget_usd: 0.50

  codex-cli:
    model: ""                  # optional; omitted if unset
    binary: codex
```

API keys are resolved as: the env var named by `api_key_env` (if set) →
the provider's default env var (`ANTHROPIC_API_KEY` / `OPENAI_API_KEY`) →
the config file's `api_key` fallback. The actual key value is never printed
by `config show`. `claude-cli` and `codex-cli` have no `api_key`-shaped
fields at all — see below.

### CLI-based authentication (claude-cli / codex-cli)

If you already pay for Claude Pro/Max or ChatGPT Plus/Pro, `provider:
claude-cli` and `provider: codex-cli` let smartly shell out to your own
logged-in `claude`/`codex` CLI session instead of a separate billed API
key. Run `claude login` / `codex login` yourself first — smartly does not
manage that login state, and if the binary is missing or the session isn't
authenticated, it hard-fails with an actionable error rather than silently
falling back to an API key.

These two providers have real, different tradeoffs from `anthropic`/`openai`:

- **Slower.** Every request spawns a full CLI process, not a lightweight
  API call.
- **Heuristic error classification.** Without HTTP status codes, failures
  (auth vs. other) are detected via exit code plus substring-matching on
  the CLI's own text output, not a precise typed error like the SDK-based
  providers use.
- **`claude-cli` is fully tool-disabled and effectively single-shot.** It
  always runs with `--safe-mode` (OAuth auth without loading your
  CLAUDE.md/hooks/plugins/MCP servers) and `--tools ""` (no tool access at
  all — verified: asked to run a command anyway, it just declines in
  text). `--model` is required in config for this reason: omitting it
  triggers an internal multi-model routing step that makes it unclear
  which model actually produced the result.
- **`codex-cli` is sandboxed but still agentic.** There is no flag to
  disable tool/shell execution entirely — it always runs with `--sandbox
  read-only`, a real OS-enforced boundary (verified: an attempted file
  write failed with a permission error and nothing was written), but the
  model may still attempt sandboxed read-only shell commands while
  composing its answer. Occasionally its response narrates a failed
  self-attempted command rather than being a clean answer — smartly's
  usual strict single-line sanitizer is the backstop here.

### Execution mode

- `auto` (default): generate and run immediately, no prompt.
- `confirm`: print the command and ask `[y/N]` before running it. Reads
  from `/dev/tty` directly, so it works even when stdin is otherwise in
  use. If no controlling terminal is available (CI, cron, a fully
  non-interactive pipe), it **fails closed** — it will not run the command —
  rather than hang or silently proceed. Use `-y`/`--yes` in those contexts.

Per-invocation overrides: `--confirm` forces the prompt for one call,
`-y`/`--yes` forces auto-run for one call (these two are mutually
exclusive), and `--dry-run` prints the command without asking or running
it at all.

### Context levels

- `none`: only your sentence is sent.
- `light` (default): adds a capped directory listing plus git
  branch/status/worktree info, so requests like "remove all worktrees
  except main" can be resolved against what's actually there.
- `full`: `light` plus a tail of your recent shell history (`$HISTFILE`, or
  `~/.zsh_history` / `~/.bash_history` by shell).

**`full` is never the default and you should turn it on deliberately.**
Your shell history can contain secrets typed inline (tokens, passwords
pasted into a command), and enabling `full` context sends that history to
a third-party LLM API on every request.

### Logging

Every generate-and-run invocation is appended to `log.path` as JSONL — one
`request` record (sentence, provider, model, generated command, outcome)
and, once the command's exit code is known, a separate `completion` record
correlated by `request_id`. The log is append-only (nothing is ever
rewritten in place) and the file is created with `0600` permissions.

**This log stores your raw sentences and generated commands verbatim**,
which may include anything sensitive you typed — treat it like shell
history.

## Flags

```
smartly <sentence...>

--provider string     override the configured provider (anthropic|openai|claude-cli|codex-cli)
--model string          override the configured model for the active provider
--context string        override the configured context level (none|light|full)
--confirm                force a confirmation prompt for this call
-y, --yes                 force auto-run for this call
--dry-run                  show the generated command without asking or running it
--print-only               internal: used by the shell wrapper, not meant for manual use
--record-exit int          internal: used by the shell wrapper, not meant for manual use

smartly init bash|zsh
smartly config init|show|path
```

## Limitations

- Generates a single shell command line per invocation (pipes, `&&`,
  redirects within that line are fine; multi-step scripts are not).
- Targets bash/zsh on Linux and macOS. Windows/PowerShell isn't supported
  yet.
- The OpenAI provider ships with no default model — you must set
  `providers.openai.model` yourself.
- `claude-cli`/`codex-cli` require the respective CLI installed and
  separately logged in (`claude login` / `codex login`) — smartly does not
  manage that login state — and their error classification is best-effort
  (see [CLI-based authentication](#cli-based-authentication-claude-cli--codex-cli)).
