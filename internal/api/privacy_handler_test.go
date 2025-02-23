package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"norma/internal/config"
)

func TestPrivacyPurge(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{}
	cfg.App.LogDir = filepath.Join(dir, "logs")
	cfg.Database.Path = filepath.Join(dir, "norma.db")
	cfg.Security.PurgeProofDir = filepath.Join(dir, "proofs")
	cfg.LLM.Provider = "ollama"
	cfg.LLM.Endpoint = "http://127.0.0.1:11434"
	cfg.TTS.Provider = "mock"
	cfg.ASR.Provider = "mock"
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	h := NewPrivacyHandler(cfgPath)
	mux := http.NewServeMux()
	h.Register(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/privacy/purge", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
