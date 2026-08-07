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

### Shell wrapper + logging correlation (`internal/shellinit`, `internal/logging`)

`smartly init bash|zsh` emits a function (embedded via `go:embed`) that the user sources. It calls `smartly --print-only` (generates + prints the command, doesn't run it) and then `eval`s the result in the *parent* shell — this is how a generated `cd`/`export` actually affects the calling shell, since a subprocess can't do that on its own. Because `--print-only` and the wrapper's subsequent `--record-exit` call are two separate `smartly` processes, they can't share Go memory to correlate a request with its outcome — the wrapper generates a request ID itself and passes it via the `SMARTLY_REQUEST_ID` env var to both calls.

The history log (`internal/logging`) is **strictly append-only**: a `request` record and a `completion` record are two separate JSONL lines joined by `request_id`, never a rewrite of an existing line. This was a deliberate design correction (an earlier read-modify-write approach was rejected as unsafe under concurrent shells) — don't reintroduce in-place log mutation.

### Config (`internal/config`)

`Load()` merges `config.yaml` onto `Defaults()` — a field absent from the file keeps its Go-side default rather than zeroing out. API key precedence for `anthropic`/`openai` is: env var named by `api_key_env` → the provider's hardcoded default env var → the config file's `api_key` fallback (`ResolveAPIKey`). `claude-cli`/`codex-cli` have no `api_key`-shaped fields at all by design. `expandHome` exists because YAML values are read literally — unlike a shell argument, nothing expands a `~/...` path in `log.path` for you.

### Testing conventions

All tests are pure — no live network calls or subprocess invocations run in `go test`. The CLI-provider tests (`claudecli_test.go`, `codexcli_test.go`) use fixture strings captured from real live invocations (embedded as consts, with comments noting when a fixture was invented vs. actually captured) rather than mocking the subprocess layer. `exec.LookPath` failure paths are tested by pointing `PATH` at an empty temp dir via `t.Setenv`.
