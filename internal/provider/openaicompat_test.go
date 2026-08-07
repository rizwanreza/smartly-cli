package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// The `openai` provider doubles as the entry point for any OpenAI-compatible
// endpoint — Fireworks, Together, Groq, vLLM, LM Studio, Azure OpenAI — via
// base_url plus an api_key_env naming that vendor's own variable. That is a
// documented capability (README, docs/providers), so it needs a test, or the
// docs are just a claim.
//
// This is the one test in the suite that binds a socket. It is still pure in
// the sense the testing conventions care about: httptest is loopback only, so
// there is no external host, no DNS, no credentials and no flakiness — unlike
// a real API call, which is what "no live network calls" is protecting against.
func TestOpenAICompatibleEndpoint(t *testing.T) {
	var gotPath, gotAuth, gotModel string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		gotModel = body.Model

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x","object":"chat.completion","model":"`+body.Model+
			`","choices":[{"index":0,"message":{"role":"assistant","content":"ls -lahS"},`+
			`"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":5}}`)
	}))
	defer srv.Close()

	// A third-party key in its own variable, with OPENAI_API_KEY deliberately
	// empty: reaching a non-OpenAI vendor must not require overloading OpenAI's
	// variable name.
	t.Setenv("FIREWORKS_API_KEY", "fw-secret-123")
	t.Setenv("OPENAI_API_KEY", "")

	const model = "accounts/fireworks/models/llama-v3p1-70b-instruct"

	cfg := config.Defaults()
	cfg.Provider = "openai"
	cfg.Providers.OpenAI.Model = model
	cfg.Providers.OpenAI.APIKeyEnv = "FIREWORKS_API_KEY"
	// Written without a trailing slash on purpose — the docs tell people either
	// form is fine, and this is what pins that.
	cfg.Providers.OpenAI.BaseURL = srv.URL + "/inference/v1"

	p, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFromConfig: %v", err)
	}

	res, err := p.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "system",
		UserPrompt:   "show hidden files sorted by size",
		MaxTokens:    100,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if want := "/inference/v1/chat/completions"; gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
	if want := "Bearer fw-secret-123"; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
	if gotModel != model {
		t.Errorf("model sent = %q, want %q (vendor model ids must pass through verbatim)", gotModel, model)
	}
	if want := "ls -lahS"; res.RawText != want {
		t.Errorf("RawText = %q, want %q", res.RawText, want)
	}
	if res.InputTokens != 11 || res.OutputTokens != 5 {
		t.Errorf("tokens = %d in / %d out, want 11 / 5", res.InputTokens, res.OutputTokens)
	}
}

// A base_url that already ends in a slash must produce the same request path as
// one that does not, since the docs tell people not to worry about it.
func TestOpenAICompatibleBaseURLTrailingSlash(t *testing.T) {
	for _, suffix := range []string{"/inference/v1", "/inference/v1/"} {
		t.Run(suffix, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"x","object":"chat.completion","model":"m","choices":`+
					`[{"index":0,"message":{"role":"assistant","content":"ls"},"finish_reason":"stop"}]}`)
			}))
			defer srv.Close()

			t.Setenv("OPENAI_API_KEY", "k")

			cfg := config.Defaults()
			cfg.Provider = "openai"
			cfg.Providers.OpenAI.Model = "m"
			cfg.Providers.OpenAI.BaseURL = srv.URL + suffix

			p, err := NewFromConfig(cfg)
			if err != nil {
				t.Fatalf("NewFromConfig: %v", err)
			}
			if _, err := p.Generate(context.Background(), GenerateRequest{
				SystemPrompt: "s", UserPrompt: "u", MaxTokens: 10,
			}); err != nil {
				t.Fatalf("Generate: %v", err)
			}

			if want := "/inference/v1/chat/completions"; gotPath != want {
				t.Errorf("request path = %q, want %q", gotPath, want)
			}
		})
	}
}
