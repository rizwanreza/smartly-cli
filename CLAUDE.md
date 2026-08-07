# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`smartly` is a Go CLI that turns an English sentence into a single executable shell command via an LLM, then (by default) runs it immediately with no confirmation. See README.md for the full user-facing feature set, config schema, and flags.

## Commands

```
go build ./...                         # build everything
go vet ./...                           # static checks — run before considering work done
go test ./...                          # full test suite (all packages are pure unit tests, no live API/subprocess calls)
go test ./internal/provider/... -run TestBuildClaudeArgs -v   # single test
go test ./internal/classify/ -run FuzzClassify -fuzz FuzzClassify -fuzztime 30s   # classifier totality fuzz
go install ./cmd/smartly               # install locally as `smartly` (see "Binary name" below for why this path matters)
goreleaser check                       # validate .goreleaser.yaml
goreleaser release --snapshot --clean --skip=publish   # local cross-compile dry run, writes to dist/
```

There is no lint config beyond `go vet`.

## Architecture

### Binary name is load-bearing

The module is `smartly-cli` but the binary must be literally named `smartly`. `go install` names the output after the *directory* containing `main.go`, not the module — so `main.go` lives at `cmd/smartly/main.go`, not the repo root. Don't move it back; that was a real bug caught mid-build. `cmd/smartly/main.go` is a thin wrapper that just calls `internal/cli.Execute()`.

### Provider abstraction (`internal/provider`)

Every LLM backend implements `Provider` (`Name() string`, `Generate(ctx, GenerateRequest) (*GenerateResult, error)`), selected by `provider.NewFromConfig` switching on `cfg.Provider`. There are four:
- `anthropic`, `openai` — official SDKs, API-key auth.
- `claude-cli`, `codex-cli` — shell out to the user's own logged-in `claude`/`codex` CLI instead of an API key (see `execcli.go`, `claudecli.go`, `codexcli.go`).

Every provider maps its own failures into the shared `provider.Error{Kind, Message, Cause}` taxonomy (`ErrKindAuth/RateLimit/Overloaded/Network/Invalid/Unknown`) so `internal/cli` never imports an SDK or parses CLI-specific error shapes directly. For the two CLI-based providers this mapping is necessarily heuristic (exit code + substring matching via `looksLikeAuthFailure`, no HTTP status codes) — that's documented as an accepted limitation, not something to "fix" by making it look more precise than it is.

**`execcli.go`'s `runCLI` has a non-obvious safety mechanism, don't simplify it away**: the child process runs in its own process group (`Setpgid`) and `runCLI` unconditionally kills that whole group after `cmd.Run()` returns, and `cmd.WaitDelay` is set. This exists because of a verified live failure: asked to generate a `tail -f` command, codex's model actually *executed* it inside its read-only sandbox rather than just describing it; `tail -f` never exits on its own, so the orphaned grandchild kept smartly's stdout pipe open and hung the whole process indefinitely even after the direct child (`codex`) had already exited. `WaitDelay` alone unblocks `cmd.Wait()`; the explicit post-`Run()` process-group kill is what actually stops the orphan from running forever in the background. Any new CLI-based provider must go through `runCLI`, not raw `exec.Command`.

Anthropic's provider sets `thinking: {type: "disabled"}` **explicitly**, not by omission — omitting it on Sonnet-5-generation-and-later models enables adaptive thinking by default, which both adds latency and can consume the whole `max_tokens` budget before any command text is emitted.

### Request pipeline (`internal/cli/root.go`)

`runRoot` is the single flow every non-subcommand invocation goes through: load config → apply `--provider`/`--model`/`--context` overrides → gather context → build prompt → `Provider.Generate` → `prompt.Sanitize` → confirm gate (if applicable) → log → execute or print. Key invariants to preserve when touching this:
- `prompt.Sanitize` hard-rejects any output containing an embedded newline — there is deliberately no "take the last line" salvage logic. An unclean model response is treated as a contract violation, not something to guess around.
- The confirmation gate reads/writes `/dev/tty` directly, never stdin/stdout — stdin may already be consumed by a command-substitution capture in the shell wrapper (see below), and it fails closed (refuses to run) if no controlling terminal is available.
- `execution.mode: auto` (run immediately, no prompt) is the default and is intentional product behavior, not a missing safety check — see README's "Auto-run is the default" callout before adding friction here.

### Destructive-command classifier (`internal/classify`)

Backs `execution.mode: confirm-destructive`. `Classify(command string) Result` is a **pure function of the string** — no `os.Stat`, no `exec.LookPath`, no subprocess, no second LLM call. That's what keeps it TOCTOU-free and testable under the no-subprocess-in-tests rule; don't "improve" it by having it check whether a path exists.

Tri-state: `Safe` / `Destructive` / `Unknown`, ordered so a command's risk is the max over its segments. **`Unknown` confirms.** That's the load-bearing decision — a seatbelt that silently passes what it doesn't recognize is worse than no seatbelt — and the accepted cost (prompting on unrecognized binaries) is documented in the README, not something to fix by defaulting Unknown to run.

`lex.go` is a shell-shaped lexer, not a shell parser. It exists to answer three things the rule tables can't: which words are in *command position* (so `echo "rm -rf /"` is Safe), where one command ends and the next begins (`| && || ; &` outside quotes), and which `>` is a write vs. an fd dup (`2>&1` is not a write; `/dev/null`-style targets are carved out). It is deliberately forgiving of malformed input — `Classify` must be **total**, which the fuzz target enforces along with "any non-Safe result carries a non-empty Reason".

`rules.go` holds four tables checked in order: always-destructive, flag-gated (`sed -i`, `find -delete`, `curl -o`, `tar -x`…), subcommand tables (git/docker/kubectl/systemctl/npm/brew/go/terraform…), and the safe allowlist. **`safeCommands` is the only table where an addition can create a false negative** — everything absent from all four falls through to Unknown, which asks. `sudo` is unconditionally Destructive (including `sudo ls`); `$(…)`, backticks, `eval` and `sh -c` floor at Unknown but still have their bodies analyzed recursively, so a destructive body wins.

This is documented as a best-effort seatbelt with possible false negatives. Keep the honesty in the copy — the README, the config template comment, and `onboard`'s mode description all say so, and the live classifier demo in `smartly onboard` exists precisely so users see it shrug at `frobnicate --all` before trusting it.

### Execution modes and the confirm gate

`config.ExecutionModes()` / `ValidateExecutionMode` are the single source of truth for `auto | confirm | confirm-destructive`; `resolveMode` in `internal/cli/root.go` defers to them rather than carrying its own switch, and there is deliberately no normalization (a `"Confirm"` typo is a fail-closed error). `--confirm`/`-y` short-circuit *before* the config mode is consulted, which is what makes "--confirm always asks, -y never asks" literally true — don't move the classifier ahead of that check.

`modeAsks(mode, verdict)` is the one place mode and classifier meet. The verdict is computed and written to the log's `risk` field on **every** request regardless of mode, so `auto` users can still audit what ran without a prompt.

`confirmExecution` takes a `confirmPrompt{Command, Mode, Reason}`; `renderConfirmPrompt` is split out so the copy is testable without a controlling terminal. The `/dev/tty` mechanics and fail-closed behavior are unchanged from the original gate — `openTTY()` is just extracted for reuse by `onboard`.

### `smartly onboard` and the one TUI exception (`internal/onboard`, `internal/cli/onboard*.go`)

`internal/onboard` holds the **pure decision logic** — `Questions` (which steps run given the answers so far), `Apply` (answers + base config → config), `Validate`, `SeedFromConfig`, `Detector` (PATH/env lookups behind injectable funcs), `BackupExisting`, and all the user-facing copy. It imports nothing terminal-shaped, which is what lets the invariants be tested without a tty: no key-shaped question exists anywhere in the flow, `Apply` refuses `context: full` without `FullContextConfirmed`, and `Answers` has no field that could hold a credential.

`internal/cli/onboard.go` + `onboard_ui.go` are the presentation layer and **the only place in the codebase that draws a form** (`github.com/charmbracelet/huh`, bound to `/dev/tty` via `WithInput`/`WithOutput`). This is a deliberate, scoped exception: the core generate-and-run pipeline stays TUI-free because it has to work with stdin consumed by the shell wrapper's command substitution, and the confirm gate is intentionally three lines of `bufio`. **Don't pull huh/bubbletea into `root.go` or `confirm.go`.**

Safety rules baked into the flow, in order of how badly it would go to break them: no API key is ever asked for, echoed or written (env-var detection only; a key already in the user's file is carried through on write but replaced with a placeholder in anything printed to the terminal); `context: full` requires a second explicit confirmation stating the consequence; rc files are never edited, only the eval line is printed; an existing config is backed up (timestamped, 0600) and pre-seeds the answers; the final write is confirmed, and declining prints the YAML and writes nothing; no tty fails closed pointing at `smartly config init`. `execution.mode: auto` remains the compiled-in default — onboarding changes nothing for anyone who never runs it.

Brand in onboard specifically: huh is themed from `ThemeBase()` in `onboardTheme()`, and no classifier verdict is ever rendered red — being asked is not a failure. Everything else (palette, symbols, voice) comes from the brand guideline; see the Brand section below.

### Config template rendering (`internal/cli/template.go`)

`config init` and `onboard` both write config.yaml through `renderConfigTemplate(cfg)`. There is exactly one copy of the file's prose, because those comments carry safety warnings (the shell-history warning on `context: full`, the verbatim-storage warning on the log, "no api_key field exists" on the CLI providers) and two drifting copies would be a silent regression. The round-trip test (render → unmarshal → `DeepEqual`) plus the assertion that each security comment survives is what pins it; `config.ContractHome` is the inverse of `expandHome` so `log.path` is written back as `~/...`.

### Shell wrapper + logging correlation (`internal/shellinit`, `internal/logging`)

`smartly init bash|zsh` emits a function (embedded via `go:embed`) that the user sources. It calls `smartly --print-only` (generates + prints the command, doesn't run it) and then `eval`s the result in the *parent* shell — this is how a generated `cd`/`export` actually affects the calling shell, since a subprocess can't do that on its own. Because `--print-only` and the wrapper's subsequent `--record-exit` call are two separate `smartly` processes, they can't share Go memory to correlate a request with its outcome — the wrapper generates a request ID itself and passes it via the `SMARTLY_REQUEST_ID` env var to both calls.

Because the wrapper shadows the binary with a shell function that `eval`s its stdout, **it must only do that for invocations that actually generate a command**. Everything else smartly prints — `--help`, `--version`, `config show`, `onboard`, completions — is prose for a human, and eval-ing it ranged from a shell parse error to, in the `--version` case, re-entering the wrapper as a prompt and making a real API call for a request nobody typed. The templates therefore open with a `case` guard that hands those straight to `command smartly "$@"`, and `shellinit.Render` takes the subcommand list as an argument so the **cobra tree stays the single source of truth** — `rootSubcommandNames()` in `internal/cli/init.go` derives it live, materialising cobra's lazily-added `help`/`completion` first. Adding a subcommand without that list updating is a silent, nasty failure (the name gets sent to the model and the reply gets eval'd), so `TestWrapperCoversEverySubcommand` asserts the rendered script covers every registered command *before* the eval line. Don't replace that with a hand-written list, and don't let `--print-only`/`--record-exit` into the guard or the wrapper recurses instead of generating.

The history log (`internal/logging`) is **strictly append-only**: a `request` record and a `completion` record are two separate JSONL lines joined by `request_id`, never a rewrite of an existing line. This was a deliberate design correction (an earlier read-modify-write approach was rejected as unsafe under concurrent shells) — don't reintroduce in-place log mutation.

### Config (`internal/config`)

`Load()` merges `config.yaml` onto `Defaults()` — a field absent from the file keeps its Go-side default rather than zeroing out. API key precedence for `anthropic`/`openai` is: env var named by `api_key_env` → the provider's hardcoded default env var → the config file's `api_key` fallback (`ResolveAPIKey`). `claude-cli`/`codex-cli` have no `api_key`-shaped fields at all by design. `expandHome` exists because YAML values are read literally — unlike a shell argument, nothing expands a `~/...` path in `log.path` for you.

### Website (`site/`, deployed to https://smartlycli.com)

Astro static build, no client framework, no analytics, no third-party requests — the fonts are self-hosted and a strict "no external URLs at runtime" posture is part of the design, so don't add a CDN script or a webfont link.

Every deployment-shaped value lives in `site/site.config.mjs`, and **nothing about the deploy target is hardcoded in the source**. `BASE_PATH` and `SITE_URL` are read from the environment; `.github/workflows/pages.yml` passes `${{ steps.pages.outputs.base_path }}` and `origin` from `actions/configure-pages`, which follow whatever the Pages site is actually configured as. That is why moving from project pages (`rizwanreza.github.io/smartly-cli`) to the apex needed no workflow change at all — only the local-build defaults, which now match production at `/` and `https://smartlycli.com`. If you find yourself typing the domain into a component, that's the bug.

`BASE_PATH` feeds three separate things — Astro's `base`, `withBase()` in `src/lib/url.ts`, and the `rehype-base-urls` plugin that rewrites root-relative links written inside Markdown — so a new internal link has to go through one of them, never a bare `/docs/...` in an `.astro` file. `npm run build:subpath` builds the whole site under `/smartly-cli` and exists purely so that machinery keeps being exercised now that production doesn't use it; run it after touching anything URL-shaped.

`npm run build` is `astro build` followed by `scripts/check-links.mjs`, which fails on any broken internal link, heading fragment or asset, and on any internal URL missing the configured base path. It gates the deploy, so a link typo fails CI rather than shipping.

Astro majors are deliberately **not** ignored in `.github/dependabot.yml` — they were once, and the site drifted two majors behind until a batch of high-severity advisories had no non-breaking fix. The `site` job in `ci.yml` builds every PR, so a breaking bot major fails visibly instead of merging quietly. Relatedly: regenerate `site/package-lock.json` with a full `rm -rf node_modules package-lock.json && npm install`, not an incremental one — a macOS-only install silently drops sharp's `@emnapi/*` wasm fallbacks and `npm ci` then refuses to install on the Linux runner.

### Brand (`docs/BRAND.md` is the authority)

**Read `docs/BRAND.md` before writing or changing anything user-facing** — CLI output copy, help text, error messages, README, the website, or docs. It defines the palette (and which colors are allowed to mean what: cyan for identity/success, amber for consequence only, red for failure only), the typed logo `smartly >_` and its rules, the terminal symbols (`›` `→` `$` `!` `·`) and when each is used, voice principles with good/bad copy pairs, and naming conventions. Highlights that get violated most easily: sentence case everywhere; lowercase `smartly` in prose; one wink per page and never around risk; never rely on color alone; no branding in JSONL logs or any machine-readable output.

The guideline is deliberately **not published on the website** — it was removed from the public site and lives only in this repo. Don't re-add a brand page to `site/` without being asked; the designed page is recoverable from git history (`site/src/pages/brand.astro`) if that decision is ever reversed. In Go code, the brand's terminal implementation is `internal/brand` (color capability detection, the `Printer`, status symbols) — extend that package rather than emitting ANSI or lipgloss styles elsewhere; the one sanctioned exception is onboard's tty-only styles in `internal/cli/onboard_ui.go`.

### Testing conventions

All tests are pure — no live network calls or subprocess invocations run in `go test`. Nothing in the suite needs a controlling terminal either: the tty-only paths (`confirmExecution`, `smartly onboard`) are covered by asserting they **fail closed** without one, and their decision logic and rendered copy are extracted into pure functions (`renderConfirmPrompt`, `dryRunNote`, everything in `internal/onboard`) so the behavior that matters is testable without a pty. Tests that would need a real terminal `t.Skip` rather than assert on unreachable behavior. The CLI-provider tests (`claudecli_test.go`, `codexcli_test.go`) use fixture strings captured from real live invocations (embedded as consts, with comments noting when a fixture was invented vs. actually captured) rather than mocking the subprocess layer. `exec.LookPath` failure paths are tested by pointing `PATH` at an empty temp dir via `t.Setenv`.
