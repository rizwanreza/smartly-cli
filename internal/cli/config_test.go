package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rizwanreza/smartly-cli/internal/config"
)

// runCLI executes rootCmd with the given args and returns combined
// stdout/stderr, restoring the command's output streams and flags
// afterward so tests don't leak state into each other.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out, err := runCLI(t, "config", "path")
	if err != nil {
		t.Fatalf("config path error = %v", err)
	}
	want := config.Path()
	if strings.TrimSpace(out) != want {
		t.Errorf("config path = %q, want %q", strings.TrimSpace(out), want)
	}
}

func TestConfigInit_WritesTemplateAndRefusesOverwrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := runCLI(t, "config", "init"); err != nil {
		t.Fatalf("first config init error = %v", err)
	}

	path := config.Path()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written config: %v", err)
	}
	if !strings.Contains(string(data), "provider: anthropic") {
		t.Errorf("written config missing expected default content, got:\n%s", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file perm = %o, want 0600", perm)
	}

	dirInfo, err := os.Stat(config.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir perm = %o, want 0700", perm)
	}

	if _, err := runCLI(t, "config", "init"); err == nil {
		t.Error("second config init should refuse to overwrite an existing file")
	}
}

func TestConfigShow_NeverPrintsSecretValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("ANTHROPIC_API_KEY", "")

	confDir := filepath.Join(dir, "smartly")
	if err := os.MkdirAll(confDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yamlContent := "provider: anthropic\nproviders:\n  anthropic:\n    api_key: super-secret-value\n"
	if err := os.WriteFile(filepath.Join(confDir, "config.yaml"), []byte(yamlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "config", "show")
	if err != nil {
		t.Fatalf("config show error = %v", err)
	}
	if strings.Contains(out, "super-secret-value") {
		t.Errorf("config show leaked the raw API key, got:\n%s", out)
	}
	if !strings.Contains(out, "<set in config>") {
		t.Errorf("config show should report the key as set-in-config, got:\n%s", out)
	}
}

func TestConfigShow_ReportsEnvVarSource(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-does-not-matter")

	out, err := runCLI(t, "config", "show")
	if err != nil {
		t.Fatalf("config show error = %v", err)
	}
	if strings.Contains(out, "sk-ant-does-not-matter") {
		t.Errorf("config show leaked the raw API key, got:\n%s", out)
	}
	if !strings.Contains(out, "<set via ANTHROPIC_API_KEY>") {
		t.Errorf("config show should report the env var source, got:\n%s", out)
	}
}
