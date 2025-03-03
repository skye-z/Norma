package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"norma/internal/config"
)

func TestProviderStatusAndInit(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &config.Config{}
	cfg.App.Name = "Norma"
	cfg.LLM.Provider = "ollama"
	cfg.LLM.Endpoint = "http://127.0.0.1:11434"
	cfg.ASR.Provider = "whispercpp"
	cfg.TTS.Provider = "piper"
	cfg.Audio.WakeProvider = "sherpa-onnx"
	cfg.RAG.EmbedProvider = "ollama"
	cfg.Providers.Root = filepath.Join(t.TempDir(), "providers")
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	h := NewProviderHandler(configPath)
	mux := http.NewServeMux()
	h.Register(mux)

	initRec := httptest.NewRecorder()
	initReq := httptest.NewRequest(http.MethodPost, "/providers/init", nil)
	mux.ServeHTTP(initRec, initReq)
	if initRec.Code != http.StatusOK {
		t.Fatalf("init code=%d body=%s", initRec.Code, initRec.Body.String())
	}

	statusRec := httptest.NewRecorder()
	statusReq := httptest.NewRequest(http.MethodGet, "/providers/status", nil)
	mux.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code=%d body=%s", statusRec.Code, statusRec.Body.String())
	}

	var out providerStatusResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Stack["wake"] != "sherpa-onnx" || out.Stack["asr"] != "whispercpp" {
		t.Fatalf("unexpected stack: %+v", out.Stack)
	}
	if out.Root == "" {
		t.Fatalf("expected root")
	}
	if _, ok := out.Providers["llm"]; !ok {
		t.Fatalf("expected llm provider bucket")
	}
}
