package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"norma/backend/utils"
	"norma/backend/core/runtime/bus"
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

func TestSettingsRuntimePutSyncsWakeKeywordFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	seed := &config.Config{}
	loaded, err := config.Load(filepath.Join(dir, "does-not-exist.yaml"))
	if err == nil {
		seed = loaded
	}
	seed.LLM.Provider = "ollama"
	seed.LLM.Endpoint = "http://127.0.0.1:11434/v1"
	seed.Audio.EnableWakeWord = true
	seed.Audio.WakeMode = "wake_word_vad"
	seed.Audio.WakeProvider = "sherpa-onnx"
	seed.Providers.Root = filepath.Join(dir, "providers")
	wakeDir := filepath.Join(seed.Providers.Root, "wake", "current")
	if err := os.MkdirAll(wakeDir, 0o755); err != nil {
		t.Fatalf("mkdir wake dir: %v", err)
	}
	for _, name := range []string{"encoder.onnx", "decoder.onnx", "joiner.onnx", "tokens.txt"} {
		if err := os.WriteFile(filepath.Join(wakeDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := config.Save(cfgPath, seed); err != nil {
		t.Fatalf("save seed: %v", err)
	}

	h := NewSettingsHandler(cfgPath, bus.New())
	mux := http.NewServeMux()
	h.Register(mux)

	seed.Audio.WakeKeyword = "诺玛"
	body, _ := json.Marshal(seed)
	putReq := httptest.NewRequest(http.MethodPut, "/settings/runtime", bytes.NewReader(body))
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put code=%d body=%s", putRec.Code, putRec.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(wakeDir, "keywords.txt"))
	if err != nil {
		t.Fatalf("read keywords: %v", err)
	}
	if string(data) != "n uò m ǎ @诺玛\n" {
		t.Fatalf("unexpected keywords content: %q", string(data))
	}

	after, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if after.Audio.WakeKeywordsFilePath == "" {
		t.Fatalf("expected wake_keywords_file_path to be persisted")
	}
}
