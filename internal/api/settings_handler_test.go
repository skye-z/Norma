package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"norma/internal/config"
	"norma/internal/runtime/bus"
)

func TestSettingsRuntimeGetPut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	seed := &config.Config{}
	loaded, err := config.Load(filepath.Join(dir, "does-not-exist.yaml"))
	if err == nil {
		seed = loaded
	}
	if err := config.Save(cfgPath, seed); err != nil {
		t.Fatalf("save seed: %v", err)
	}

	h := NewSettingsHandler(cfgPath, bus.New())
	mux := http.NewServeMux()
	h.Register(mux)

	getReq := httptest.NewRequest(http.MethodGet, "/settings/runtime", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get code=%d body=%s", getRec.Code, getRec.Body.String())
	}

	var cfg config.Config
	if err := json.Unmarshal(getRec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	cfg.LLM.Endpoint = "http://127.0.0.1:11435"
	cfg.LLM.Provider = "ollama"
	cfg.RAG.TopK = 8

	body, _ := json.Marshal(cfg)
	putReq := httptest.NewRequest(http.MethodPut, "/settings/runtime", bytes.NewReader(body))
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put code=%d body=%s", putRec.Code, putRec.Body.String())
	}

	after, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload after put: %v", err)
	}
	if after.LLM.Endpoint != "http://127.0.0.1:11435" {
		t.Fatalf("endpoint not updated: %q", after.LLM.Endpoint)
	}
	if after.RAG.TopK != 8 {
		t.Fatalf("rag top_k not updated: %d", after.RAG.TopK)
	}
}
