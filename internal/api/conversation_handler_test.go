package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"norma/internal/config"
	"norma/internal/runtime"
	"norma/internal/runtime/bus"
)

func TestConversationMicStartStopInterrupt(t *testing.T) {
	rt := runtime.New()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := config.Save(configPath, &config.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	h := NewConversationHandler(configPath, rt.Bus, rt.Inspector)
	mux := http.NewServeMux()
	h.Register(mux)

	seen := map[string]int{}
	_ = rt.Bus.SubscribeAll(func(evt bus.Event) {
		seen[evt.Name] += 1
	})

	startRec := httptest.NewRecorder()
	startReq := httptest.NewRequest(http.MethodPost, "/conversation/mic/start", bytes.NewReader([]byte(`{}`)))
	mux.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start code=%d body=%s", startRec.Code, startRec.Body.String())
	}

	var state map[string]any
	if err := json.Unmarshal(startRec.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if state["active"] != true {
		t.Fatalf("expected active=true, got=%v", state["active"])
	}

	deadline := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(deadline) && seen["audio.vad.start"] == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if seen["audio.vad.start"] == 0 {
		t.Fatalf("expected audio.vad.start event")
	}

	stopRec := httptest.NewRecorder()
	stopReq := httptest.NewRequest(http.MethodPost, "/conversation/mic/stop", bytes.NewReader([]byte(`{}`)))
	mux.ServeHTTP(stopRec, stopReq)
	if stopRec.Code != http.StatusOK {
		t.Fatalf("stop code=%d body=%s", stopRec.Code, stopRec.Body.String())
	}

	interruptRec := httptest.NewRecorder()
	interruptReq := httptest.NewRequest(http.MethodPost, "/conversation/mic/interrupt", bytes.NewReader([]byte(`{"reason":"test"}`)))
	mux.ServeHTTP(interruptRec, interruptReq)
	if interruptRec.Code != http.StatusOK {
		t.Fatalf("interrupt code=%d body=%s", interruptRec.Code, interruptRec.Body.String())
	}

	if seen["audio.vad.stop"] == 0 {
		t.Fatalf("expected audio.vad.stop event")
	}
	if seen["interrupt"] == 0 {
		t.Fatalf("expected interrupt event")
	}
}

func TestConversationMicTest(t *testing.T) {
	rt := runtime.New()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := config.Save(configPath, &config.Config{}); err != nil {
		t.Fatalf("save empty config: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	cfg.Audio.MicDeviceID = "default"
	cfg.Audio.MicSampleRate = 16000
	cfg.Audio.MicChannelCount = 1
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	h := NewConversationHandler(configPath, rt.Bus, rt.Inspector)
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/conversation/mic/test", bytes.NewReader([]byte(`{}`)))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok=true, got=%v", out["ok"])
	}
}
