package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveDoesNotPersistEnvInjectedAPIKey(t *testing.T) {
	t.Setenv("NORMA_LLM_API_KEY", "sk-from-env-123")
	cfg := &Config{}
	cfg.LLM.Provider = "deepseek"
	cfg.LLM.APIKey = "sk-from-env-123"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "sk-from-env-123") {
		t.Fatalf("env key leaked into config file:\n%s", body)
	}
	if cfg.LLM.APIKey != "sk-from-env-123" {
		t.Fatal("Save must not mutate the caller's config")
	}
}

func TestSaveKeepsExplicitlyConfiguredAPIKey(t *testing.T) {
	t.Setenv("NORMA_LLM_API_KEY", "")
	cfg := &Config{}
	cfg.LLM.APIKey = "sk-user-typed-in-console"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "sk-user-typed-in-console") {
		t.Fatalf("explicit key was dropped:\n%s", body)
	}
}

func TestLoadTreatsZeroTopKAsUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("rag:\n  top_k: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RAG.TopK != 6 {
		t.Fatalf("top_k = %d, want 6", cfg.RAG.TopK)
	}
}
