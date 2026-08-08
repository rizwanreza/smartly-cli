# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`smartly` is a Go CLI that turns an English sentence into a single executable shell command via an LLM, then (by default) runs it immediately with no confirmation. README.md has the user-facing feature set, config schema and flags.

## Commands

```
go build ./...                         # build everything
go vet ./...                           # static checks — run before considering work done
go test ./...                          # full suite
go test ./internal/provider/... -run TestBuildClaudeArgs -v   # single test
go test ./internal/classify/ -run FuzzClassify -fuzz FuzzClassify -fuzztime 30s   # classifier totality fuzz
go install ./cmd/smartly               # install locally as `smartly`
goreleaser check                       # validate .goreleaser.yaml
goreleaser release --snapshot --clean --skip=publish   # local cross-compile dry run → dist/
cd site && npm run build               # astro build + link check (gates the Pages deploy)
```

No lint config beyond `go vet`.

## Architecture

### Binary name is load-bearing

`go install` names the output after the *directory* holding `main.go`, not the module — so it lives at `cmd/smartly/main.go`, a thin wrapper around `internal/cli.Execute()`. Moving it to the repo root yields a binary called `smartly-cli`.

### Providers (`internal/provider`)

Four backends implement `Provider`, selected by `NewFromConfig` on `cfg.Provider`: `anthropic`/`openai` (SDKs, API key) and `claude-cli`/`codex-cli` (shell out to the user's logged-in CLI).

- Each maps failures into `provider.Error{Kind, …}` so `internal/cli` never imports an SDK. For the CLI providers that mapping is heuristic (exit code + substring, no HTTP status) — an accepted limitation, not something to dress up as precise.
- **`execcli.go`'s `runCLI`: don't simplify.** The child runs in its own process group (`Setpgid`), `WaitDelay` is set, and the group is killed unconditionally after `cmd.Run()`. Codex once *executed* a generated `tail -f` inside its sandbox; the orphan held stdout open and hung smartly indefinitely after `codex` itself had exited. `WaitDelay` unblocks `Wait()`; the group kill is what stops the orphan. New CLI providers go through `runCLI`, never raw `exec.Command`.
- Anthropic sets `thinking: {type: "disabled"}` **explicitly**. Omitting it enables adaptive thinking on Sonnet-5-and-later, which can consume the whole `max_tokens` budget before any command text is emitted.
- The `openai` provider is also the entry point for any OpenAI-compatible endpoint (Fireworks, vLLM, …) via `base_url` + `api_key_env`. It uses Chat Completions, not the Responses API, because compatible servers implement the former.

### Request pipeline (`internal/cli/root.go`)

`runRoot` is the single flow: load config → apply `--provider`/`--model`/`--context` → gather context → build prompt → `Generate` → `prompt.Sanitize` → confirm gate → log → execute or print. Invariants:

- `Sanitize` hard-rejects embedded newlines. No "take the last line" salvage — an unclean response is a contract violation.
- The confirm gate uses `/dev/tty` directly, never stdin/stdout (stdin may be consumed by the wrapper's command substitution), and fails closed with no controlling terminal.
- `execution.mode: auto` is intentional product behavior, not a missing safety check. Read the README's "Auto-run is the default" callout before adding friction.

### Classifier (`internal/classify`)

Backs `confirm-destructive`. `Classify(string) Result` is a **pure function of the string** — no `os.Stat`, no `LookPath`, no subprocess, no second LLM call. That is what keeps it TOCTOU-free; don't "improve" it by checking whether a path exists.

- Tri-state `Safe`/`Destructive`/`Unknown`; risk is the max over segments. **`Unknown` confirms** — a seatbelt that silently passes what it doesn't recognize is worse than none, and prompting on unrecognized binaries is the accepted cost.
- `lex.go` is a shell-shaped lexer, not a parser. It answers three things the tables can't: what is in command position (so `echo "rm -rf /"` is Safe), where segments split (`| && || ; &` outside quotes), and which `>` is a write vs an fd dup (`2>&1` is not). It is forgiving of malformed input because `Classify` must be **total** — the fuzz target enforces that, plus "any non-Safe result carries a non-empty Reason".
- `rules.go` holds four tables checked in order: always-destructive, flag-gated (`sed -i`, `find -delete`, …), subcommand tables (git/docker/kubectl/…), and the safe allowlist. **`safeCommands` is the only table where an addition can create a false negative**; anything absent from all four falls through to Unknown. `sudo` is unconditionally Destructive (including `sudo ls`); `$(…)`, backticks, `eval` and `sh -c` floor at Unknown but have their bodies analyzed recursively.
- It is a best-effort seatbelt with possible false negatives. Keep that honesty in the README, the config template comment and onboard's mode description — the live demo in `onboard` exists so users watch it shrug at `frobnicate --all` before trusting it.

### Execution modes and the confirm gate

`config.ExecutionModes()`/`ValidateExecutionMode` are the single source of truth for `auto | confirm | confirm-destructive`; `resolveMode` defers to them. No normalization — a `"Confirm"` typo is a fail-closed error. `--confirm`/`-y` short-circuit *before* the config mode is consulted, which is what makes "--confirm always asks, -y never asks" literally true; don't move the classifier ahead of that check. `modeAsks(mode, verdict)` is the one place mode and classifier meet, and the verdict is written to the log's `risk` field on **every** request regardless of mode, so `auto` users can still audit. `renderConfirmPrompt` is split out so the copy is testable without a tty.

### `onboard` and the one TUI exception

`internal/onboard` is **pure decision logic** — `Questions`, `Apply`, `Validate`, `SeedFromConfig`, `Detector`, `BackupExisting` and all copy — and imports nothing terminal-shaped, which is what lets the invariants be tested without a tty.

`internal/cli/onboard{,_ui}.go` is the only place in the codebase that draws a form (`huh`, bound to `/dev/tty`). Scoped exception: the generate-and-run pipeline stays TUI-free because stdin belongs to the wrapper, and the confirm gate is intentionally three lines of `bufio`. **Don't pull huh/bubbletea into `root.go` or `confirm.go`.**

Safety rules, worst-to-break first: no API key is ever asked for, echoed or written (env detection only; a key already in the user's file is carried through on write but shown as a placeholder); `context: full` requires a second explicit confirmation; rc files are never edited, only the eval line printed; an existing config is backed up (timestamped, 0600) and pre-seeds the answers; the final write is confirmed, and declining writes nothing; no tty fails closed pointing at `config init`. `auto` stays the compiled-in default. No classifier verdict is ever rendered red — being asked is not a failure.

### Config template (`internal/cli/template.go`)

`config init` and `onboard` both write through `renderConfigTemplate(cfg)`. There is exactly one copy of the prose, because those comments carry safety warnings (shell history on `context: full`, verbatim storage on the log, "no api_key field exists" on the CLI providers) and two copies would drift silently. Pinned by a round-trip test (render → unmarshal → `DeepEqual`) plus an assertion that each security comment survives. `config.ContractHome` is the inverse of `expandHome`, so `log.path` is written back as `~/...`.

### Shell wrapper + logging (`internal/shellinit`, `internal/logging`)

`smartly init bash|zsh` emits a function (via `go:embed`) that the user sources. It runs `smartly --print-only` and `eval`s the result in the *parent* shell — that is how a generated `cd`/`export` affects the calling shell. `--print-only` and the follow-up `--record-exit` are separate processes, so the wrapper generates the request ID itself and passes it via `SMARTLY_REQUEST_ID` to both.

Because the wrapper shadows the binary and evals its stdout, **it must only do that for invocations that generate a command**. Everything else (`--help`, `--version`, `config show`, `onboard`, completions) is prose for a human; eval-ing it ranged from a shell parse error to `--version` re-entering the wrapper as a prompt and billing a real API call. Hence the `case` guard handing those to `command smartly "$@"`. `shellinit.Render` takes the subcommand list as an argument so the **cobra tree stays the single source of truth** — `rootSubcommandNames()` derives it live, materialising cobra's lazily-added `help`/`completion`. A stale list fails silently and badly (the name goes to the model, the reply gets eval'd), so `TestWrapperCoversEverySubcommand` pins it. Don't hand-write that list, and keep `--print-only`/`--record-exit` out of the guard or the wrapper recurses instead of generating.

The history log is **strictly append-only**: `request` and `completion` are two JSONL lines joined by `request_id`, never a rewrite. An earlier read-modify-write design was rejected as unsafe under concurrent shells; don't reintroduce it.

### Config (`internal/config`)

`Load()` merges `config.yaml` onto `Defaults()`, so a field absent from the file keeps its Go default rather than zeroing out. Key precedence for `anthropic`/`openai`: the env var named by `api_key_env` → the provider's default env var → the file's `api_key` (`ResolveAPIKey`). `claude-cli`/`codex-cli` have no key-shaped fields by design. `expandHome` exists because YAML is read literally — nothing expands a `~/...` path in `log.path` for you.

### Website (`site/`, deployed to https://smartlycli.com)

Astro static build; no client framework, analytics or third-party requests, fonts self-hosted — don't add a CDN script or a webfont link.

- **Nothing about the deploy target is hardcoded.** `BASE_PATH`/`SITE_URL` come from the environment, and `pages.yml` passes `base_path` and `origin` from `actions/configure-pages`, which follow the live Pages config — which is why moving to the apex needed no workflow change. Typing the domain into a component is the bug.
- `BASE_PATH` feeds three things — Astro's `base`, `withBase()` (`src/lib/url.ts`), and the `rehype-base-urls` plugin for Markdown links — so a new internal link goes through one of them, never a bare `/docs/...` in an `.astro` file. `npm run build:subpath` exercises the prefixed case; run it after touching anything URL-shaped.
- `npm run build` is `astro build` + `scripts/check-links.mjs`, which fails on any broken internal link, fragment or asset, and on any internal URL missing the base path.
- Astro majors are deliberately **not** ignored in dependabot — they were once, and the site drifted two majors behind until the advisories had no non-breaking fix. Regenerate `package-lock.json` with a full `rm -rf node_modules package-lock.json && npm install`; an incremental macOS install drops sharp's `@emnapi/*` wasm fallbacks and `npm ci` then fails on the Linux runner.

### Brand (`docs/BRAND.md` is the authority)

**Read `docs/BRAND.md` before writing or changing anything user-facing** — CLI copy, help text, error messages, README, the site, docs. It defines the palette and what each colour may mean (cyan identity/success, amber consequence only, red failure only), the `smartly >_` logo, the symbols `›` `→` `$` `!` `·`, voice with good/bad pairs, and naming. Most-violated rules: sentence case; lowercase `smartly` in prose; one wink per page and never around risk; never rely on colour alone; no branding in JSONL or any machine-readable output.

The guideline is deliberately **not published on the site** — don't re-add a brand page to `site/` without being asked (recoverable from git history at `site/src/pages/brand.astro`). In Go, `internal/brand` is the implementation; extend it rather than emitting ANSI or lipgloss styles elsewhere. Sole exception: onboard's tty-only styles in `onboard_ui.go`.

### Testing conventions

No live network or subprocess calls, and nothing needs a controlling terminal. The tty-only paths (`confirmExecution`, `onboard`) are covered by asserting they **fail closed** without one, with their decision logic and copy extracted into pure functions (`renderConfirmPrompt`, `dryRunNote`, all of `internal/onboard`). Tests that would need a real terminal `t.Skip` rather than assert on unreachable behavior. CLI-provider tests use fixtures captured from real invocations (consts, with comments noting invented vs captured) rather than mocking the subprocess layer. `exec.LookPath` failures are tested by pointing `PATH` at an empty temp dir via `t.Setenv`. The one sanctioned socket is `openaicompat_test.go`'s `httptest` stub — loopback only, so no external host, DNS or credentials.
