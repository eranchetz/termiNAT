package datahub

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigEmpty(t *testing.T) {
	cfg := parseConfig("")
	if cfg.APIKey != "" || cfg.CustomerContext != "" {
		t.Fatal("expected empty config")
	}
}

func TestParseConfigValid(t *testing.T) {
	content := `[datahub]
api_key = "test-key-123"
customer_context = "ctx-456"
`
	cfg := parseConfig(content)
	if cfg.APIKey != "test-key-123" {
		t.Fatalf("got APIKey=%q, want test-key-123", cfg.APIKey)
	}
	if cfg.CustomerContext != "ctx-456" {
		t.Fatalf("got CustomerContext=%q, want ctx-456", cfg.CustomerContext)
	}
}

func TestParseConfigMultipleSections(t *testing.T) {
	content := `[other]
foo = "bar"

[datahub]
api_key = "my-key"

[another]
baz = "qux"
`
	cfg := parseConfig(content)
	if cfg.APIKey != "my-key" {
		t.Fatalf("got APIKey=%q, want my-key", cfg.APIKey)
	}
}

func TestParseConfigNoQuotes(t *testing.T) {
	content := `[datahub]
api_key = plain-key
`
	cfg := parseConfig(content)
	if cfg.APIKey != "plain-key" {
		t.Fatalf("got APIKey=%q, want plain-key", cfg.APIKey)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := Config{APIKey: "save-key", CustomerContext: "save-ctx"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded := LoadConfig()
	if loaded.APIKey != "save-key" || loaded.CustomerContext != "save-ctx" {
		t.Fatalf("round-trip failed: got %+v", loaded)
	}

	// Overwrite and verify
	cfg2 := Config{APIKey: "new-key", CustomerContext: ""}
	if err := SaveConfig(cfg2); err != nil {
		t.Fatalf("SaveConfig overwrite: %v", err)
	}
	loaded2 := LoadConfig()
	if loaded2.APIKey != "new-key" {
		t.Fatalf("overwrite failed: got %+v", loaded2)
	}
}

func TestSaveConfigPreservesOtherSections(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path := filepath.Join(tmp, ".terminat", "config.toml")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("[other]\nfoo = \"bar\"\n"), 0644)

	SaveConfig(Config{APIKey: "k"})

	data, _ := os.ReadFile(path)
	content := string(data)
	if !contains(content, "[other]") || !contains(content, "foo") {
		t.Fatalf("other section lost: %s", content)
	}
	if !contains(content, "api_key") {
		t.Fatalf("datahub section missing: %s", content)
	}
}

func TestResolveAPIKeyPrecedence(t *testing.T) {
	// Flag wins
	if v := ResolveAPIKey("flag-val"); v != "flag-val" {
		t.Fatalf("flag precedence: got %q", v)
	}

	// Env wins over config
	t.Setenv("DOIT_DATAHUB_API_KEY", "env-val")
	if v := ResolveAPIKey(""); v != "env-val" {
		t.Fatalf("env precedence: got %q", v)
	}

	// Clear env, falls back to config (empty in test)
	t.Setenv("DOIT_DATAHUB_API_KEY", "")
	v := ResolveAPIKey("")
	// Just verify it doesn't panic; value depends on home dir config
	_ = v
}

func TestResolveCustomerContextPrecedence(t *testing.T) {
	if v := ResolveCustomerContext("flag"); v != "flag" {
		t.Fatalf("flag precedence: got %q", v)
	}

	t.Setenv("DOIT_CUSTOMER_CONTEXT", "env")
	if v := ResolveCustomerContext(""); v != "env" {
		t.Fatalf("env precedence: got %q", v)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
