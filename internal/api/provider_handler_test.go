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
	if out.Health["asr"].Status != "missing" || out.Health["wake"].Status != "missing" || out.Health["tts"].Status != "missing" {
		t.Fatalf("unexpected health: %+v", out.Health)
	}
}

func TestProviderStatusHealthGrading(t *testing.T) {
	root := t.TempDir()
	providersRoot := filepath.Join(root, "providers")
	mustMkdirAll(t, filepath.Join(providersRoot, "asr", "current"))
	mustMkdirAll(t, filepath.Join(providersRoot, "tts", "current"))
	mustMkdirAll(t, filepath.Join(providersRoot, "wake", "current"))
	mustWriteFile(t, filepath.Join(providersRoot, "asr", "current", "ggml-base.bin"), "model")
	mustWriteFile(t, filepath.Join(providersRoot, "tts", "current", "piper.exe"), "bin")
	mustWriteFile(t, filepath.Join(providersRoot, "tts", "current", "zh_CN-huayan-medium.onnx"), "model")
	mustWriteFile(t, filepath.Join(providersRoot, "wake", "current", "encoder-epoch-12-avg-2.onnx"), "encoder")
	mustWriteFile(t, filepath.Join(providersRoot, "wake", "current", "decoder-epoch-12-avg-2.onnx"), "decoder")
	mustWriteFile(t, filepath.Join(providersRoot, "wake", "current", "joiner-epoch-12-avg-2.onnx"), "joiner")
	mustWriteFile(t, filepath.Join(providersRoot, "wake", "current", "tokens.txt"), "tokens")

	configPath := filepath.Join(root, "config.yaml")
	cfg := &config.Config{}
	cfg.App.Name = "Norma"
	cfg.Providers.Root = providersRoot
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	h := NewProviderHandler(configPath)
	mux := http.NewServeMux()
	h.Register(mux)

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
	if out.Health["asr"].Status != "partial" {
		t.Fatalf("expected asr partial without runtime, got %+v", out.Health["asr"])
	}
	if !contains(out.Health["asr"].Missing, "runtime") {
		t.Fatalf("expected asr runtime missing, got %+v", out.Health["asr"])
	}
	if out.Health["tts"].Status != "partial" {
		t.Fatalf("expected tts partial, got %+v", out.Health["tts"])
	}
	if out.Health["wake"].Status != "partial" {
		t.Fatalf("expected wake partial, got %+v", out.Health["wake"])
	}
	if !contains(out.Health["wake"].Missing, "keywords") {
		t.Fatalf("expected wake missing keywords, got %+v", out.Health["wake"])
	}
}

func TestProviderApplyManaged(t *testing.T) {
	root := t.TempDir()
	providersRoot := filepath.Join(root, "providers")
	mustMkdirAll(t, filepath.Join(providersRoot, "asr", "current"))
	mustWriteFile(t, filepath.Join(providersRoot, "asr", "current", "whisper-cli.exe"), "bin")
	mustWriteFile(t, filepath.Join(providersRoot, "asr", "current", "ggml-base.bin"), "model")

	configPath := filepath.Join(root, "config.yaml")
	cfg := &config.Config{}
	cfg.App.Name = "Norma"
	cfg.LLM.Provider = "ollama"
	cfg.LLM.Endpoint = "http://127.0.0.1:11434"
	cfg.Providers.Root = providersRoot
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	h := NewProviderHandler(configPath)
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/providers/apply-managed", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	updated, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if filepath.Base(updated.ASR.BinaryPath) != "whisper-cli.exe" {
		t.Fatalf("unexpected asr binary: %s", updated.ASR.BinaryPath)
	}
	if filepath.Base(updated.ASR.ModelPath) != "ggml-base.bin" {
		t.Fatalf("unexpected asr model: %s", updated.ASR.ModelPath)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
