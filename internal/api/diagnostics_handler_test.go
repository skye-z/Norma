package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"norma/internal/config"
	"norma/internal/db"
	"norma/internal/memory"
	"norma/internal/runtime"
)

func TestDiagnosticsRunMemory(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	modelDir := filepath.Join(root, "model")
	if err := config.Save(configPath, &config.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	engine, err := db.NewEngine(filepath.Join(root, "api.db"), false)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer engine.Close()
	rt := runtime.New()
	h := NewDiagnosticsHandler(configPath, modelDir, rt.Bus, rt.Inspector, memory.NewService(engine))
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]any{"text": "diagnostics memory integration"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/diagnostics/run/memory", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var out diagnosticsRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.OK || out.Target != "memory" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestDiagnosticsRunLive2D(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	modelDir := filepath.Join(root, "model")
	if err := config.Save(configPath, &config.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir model dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "demo.model3.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "demo.cmo3"), []byte("stub"), 0o644); err != nil {
		t.Fatalf("write cmo3: %v", err)
	}
	rt := runtime.New()
	h := NewDiagnosticsHandler(configPath, modelDir, rt.Bus, rt.Inspector, nil)
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]any{"motion": "Tap", "eye_x": 0.3, "eye_y": -0.2})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/diagnostics/run/live2d", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var out diagnosticsRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.OK || out.Target != "live2d" {
		t.Fatalf("unexpected response: %+v", out)
	}
}
