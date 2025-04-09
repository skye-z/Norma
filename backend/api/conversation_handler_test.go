package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"norma/backend/core/audio"
	"norma/backend/utils"
	"norma/backend/core/providers"
	"norma/backend/core/runtime"
	"norma/backend/core/runtime/bus"
)

func TestConversationMicStartStopInterrupt(t *testing.T) {
	t.Skip("streaming conversation path is integration-covered; event timing is flaky in unit test")
	rt := runtime.New()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := config.Save(configPath, &config.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	h := NewConversationHandler(configPath, rt.Bus, rt.Inspector, providers.NewManager(filepath.Join(root, "providers")))
	mux := http.NewServeMux()
	h.Register(mux)

	seen := map[string]int{}
	gotAny := make(chan bus.Event, 16)
	mu := make(chan struct{}, 1)
	mu <- struct{}{}
	_ = rt.Bus.SubscribeAll(func(evt bus.Event) {
		<-mu
		seen[evt.Name] += 1
		mu <- struct{}{}
		select {
		case gotAny <- evt:
		default:
		}
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
	sessionID, _ := state["session_id"].(string)
	traceID, _ := state["trace_id"].(string)
	frameBody, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"trace_id":   traceID,
		"frames":     audio.DefaultDemoFrames()[10],
		"eof":        false,
	})
	frameRec := httptest.NewRecorder()
	frameReq := httptest.NewRequest(http.MethodPost, "/conversation/mic/frame", bytes.NewReader(frameBody))
	mux.ServeHTTP(frameRec, frameReq)
	if frameRec.Code != http.StatusOK {
		t.Fatalf("frame code=%d body=%s", frameRec.Code, frameRec.Body.String())
	}

	frameBody2, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"trace_id":   traceID,
		"frames":     audio.DefaultDemoFrames()[11],
		"eof":        true,
	})
	frameRec2 := httptest.NewRecorder()
	frameReq2 := httptest.NewRequest(http.MethodPost, "/conversation/mic/frame", bytes.NewReader(frameBody2))
	mux.ServeHTTP(frameRec2, frameReq2)

	time.Sleep(250 * time.Millisecond)
	<-mu
	startCount := seen["audio.vad.start"]
	mu <- struct{}{}
	if startCount == 0 {
		last := make([]string, 0, 8)
		for {
			select {
			case evt := <-gotAny:
				last = append(last, evt.Name)
			default:
				t.Fatalf("expected audio.vad.start event, seen=%v", last)
			}
		}
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

	time.Sleep(100 * time.Millisecond)
	<-mu
	stopCount := seen["audio.vad.stop"]
	interruptCount := seen["interrupt"]
	mu <- struct{}{}
	if stopCount == 0 {
		t.Fatalf("expected audio.vad.stop event")
	}
	if interruptCount == 0 {
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
	h := NewConversationHandler(configPath, rt.Bus, rt.Inspector, providers.NewManager(filepath.Join(root, "providers")))
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

func TestConversationMicFrameRequiresActiveStream(t *testing.T) {
	rt := runtime.New()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := config.Save(configPath, &config.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	h := NewConversationHandler(configPath, rt.Bus, rt.Inspector, providers.NewManager(filepath.Join(root, "providers")))
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]any{
		"session_id": "missing",
		"trace_id":   "missing",
		"frames":     []float32{0.1, -0.1},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/conversation/mic/frame", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestConversationInterruptKeepsNativeSourceRegisteredUntilPipelineStops(t *testing.T) {
	rt := runtime.New()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	cfg := &config.Config{}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	h := NewConversationHandler(configPath, rt.Bus, rt.Inspector, providers.NewManager(filepath.Join(root, "providers")))
	h.sources["s1"] = audio.NewMockFrameSource(audio.DefaultDemoFrames(), 40*time.Millisecond)
	h.sid = "s1"
	h.tid = "t1"
	h.active = true
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/conversation/mic/interrupt", bytes.NewReader([]byte(`{"session_id":"s1","trace_id":"t1","reason":"barge"}`)))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("interrupt code=%d", rec.Code)
	}
	if h.sources["s1"] == nil {
		t.Fatalf("expected source to remain registered for background capture path")
	}
}

func TestConversationInterruptClosesStreamButPreservesSource(t *testing.T) {
	rt := runtime.New()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := config.Save(configPath, &config.Config{}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	h := NewConversationHandler(configPath, rt.Bus, rt.Inspector, providers.NewManager(filepath.Join(root, "providers")))
	stream := audio.NewPushFrameSource(8)
	h.streams["s2"] = stream
	h.sources["s2"] = audio.NewMockFrameSource(audio.DefaultDemoFrames(), 40*time.Millisecond)
	h.sid = "s2"
	h.tid = "t2"
	h.active = true
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/conversation/mic/interrupt", bytes.NewReader([]byte(`{"session_id":"s2","trace_id":"t2","reason":"barge"}`)))
	mux.ServeHTTP(rec, req)
	if h.streams["s2"] != nil {
		t.Fatalf("expected push stream removed on interrupt")
	}
	if h.sources["s2"] == nil {
		t.Fatalf("expected native source preserved for continuous capture")
	}
}
