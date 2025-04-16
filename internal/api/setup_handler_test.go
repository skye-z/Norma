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

func TestSetupWizardReport(t *testing.T) {
	root := t.TempDir()
	providersRoot := filepath.Join(root, "providers")
	mustCreateSetupFile(t, filepath.Join(providersRoot, "wake", "current", "sherpa-onnx-keyword-spotter.exe"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "wake", "current", "encoder-epoch-12-avg-2.onnx"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "wake", "current", "decoder-epoch-12-avg-2.onnx"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "wake", "current", "joiner-epoch-12-avg-2.onnx"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "wake", "current", "tokens.txt"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "wake", "current", "keywords.txt"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "asr", "current", "whisper-cli.exe"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "asr", "current", "ggml-base.bin"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "model.onnx"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "voices.bin"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "tokens.txt"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "lexicon-us-en.txt"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "lexicon-zh.txt"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "date-zh.fst"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "number-zh.fst"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "phone-zh.fst"))
	mustCreateSetupFile(t, filepath.Join(providersRoot, "tts", "current", "espeak-ng-data", "README"))

	modelDir := filepath.Join(root, "model")
	mustCreateSetupFile(t, filepath.Join(modelDir, "runtime", "companion.model3.json"))
	mustCreateSetupFile(t, filepath.Join(modelDir, "runtime", "companion.cmo3"))

	configPath := filepath.Join(root, "config.yaml")
	cfg := &config.Config{}
	cfg.App.Name = "Norma"
	cfg.Database.Path = filepath.Join(root, "data", "norma.db")
	cfg.Providers.Root = providersRoot
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	h := NewSetupHandler(configPath, modelDir)
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/setup/wizard", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var out SetupWizardReport
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Ready {
		t.Fatalf("expected ready report, got %+v", out)
	}
	if out.Provider["asr"].Status != "ready" || out.Provider["tts"].Status != "ready" || out.Provider["wake"].Status != "ready" {
		t.Fatalf("unexpected provider health: %+v", out.Provider)
	}
	if !out.Model.OK() {
		t.Fatalf("expected valid model: %+v", out.Model)
	}
}

func mustCreateSetupFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
