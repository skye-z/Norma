package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"norma/internal/config"
	"norma/internal/providers"
)

func TestProviderExportReadiness(t *testing.T) {
	root := t.TempDir()
	providersRoot := filepath.Join(root, "providers")
	if err := os.MkdirAll(filepath.Join(providersRoot, "asr", "current"), 0o755); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	manifest := providers.Manifest{
		Version: 1,
		Providers: []providers.ManifestRecord{
			{
				ID:            "asr-whispercpp-windows-x64",
				Name:          "whisper.cpp",
				Kind:          "asr",
				Version:       "v1.8.2",
				Installed:     true,
				Source:        "https://github.com/ggml-org/whisper.cpp/releases/download/v1.8.2/whisper-bin-x64.zip",
				Integrity:     "official-source-only",
				LocalSHA256:   "abc123",
				InstalledAt:   "2026-03-07T12:00:00Z",
				IntegrityNote: "上游未提供 checksum 清单，已记录本地 SHA256。",
			},
		},
	}
	mgr := providers.NewManager(providersRoot)
	if err := mgr.SaveManifest(manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	configPath := filepath.Join(root, "config.yaml")
	cfg := &config.Config{}
	cfg.App.Name = "Norma"
	cfg.Providers.Root = providersRoot
	cfg.ASR.Provider = "whispercpp"
	cfg.TTS.Provider = "piper"
	cfg.Audio.WakeProvider = "sherpa-onnx"
	cfg.RAG.EmbedProvider = "ollama"
	cfg.LLM.Provider = "ollama"
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
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if _, ok := report["manifest"].(map[string]any); !ok {
		t.Fatalf("expected manifest in report: %v", report)
	}
	stack, _ := report["stack"].(map[string]any)
	if stack["asr"] != "whispercpp" {
		t.Fatalf("unexpected stack: %v", stack)
	}
	manifestOut := report["manifest"].(map[string]any)
	providersOut, _ := manifestOut["providers"].([]any)
	if len(providersOut) != 1 {
		t.Fatalf("expected one manifest provider, got %v", providersOut)
	}
	record, _ := providersOut[0].(map[string]any)
	if record["local_sha256"] != "abc123" {
		t.Fatalf("expected local sha256 in report, got %v", record)
	}
}
