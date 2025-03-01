package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"norma/internal/config"
)

func TestProviderExportReadiness(t *testing.T) {
	root := t.TempDir()
	providersRoot := filepath.Join(root, "providers")
	if err := os.MkdirAll(filepath.Join(providersRoot, "asr", "current"), 0o755); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}

	configPath := filepath.Join(root, "config.yaml")
	cfg := &config.Config{}
	cfg.App.Name = "Norma"
	cfg.Providers.Root = providersRoot
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	h := NewProviderHandler(configPath)
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/providers/export-readiness", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	reportPath, _ := out["path"].(string)
	if reportPath == "" {
		t.Fatalf("expected report path")
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("stat report: %v", err)
	}
}
