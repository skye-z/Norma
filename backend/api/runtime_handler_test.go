package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"norma/backend/core/runtime"
	"norma/backend/core/runtime/bus"
)

func TestRuntimePublish(t *testing.T) {
	rt := runtime.New()
	h := NewRuntimeHandler(rt.Bus, rt.Inspector)
	mux := http.NewServeMux()
	h.Register(mux)

	got := false
	_ = rt.Bus.Subscribe("demo", func(evt bus.Event) {
		_ = evt
		got = true
	})

	body, _ := json.Marshal(map[string]any{"name": "demo", "payload": "x"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/runtime/publish", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !got {
		t.Fatalf("expected event to be published")
	}
}

func TestRuntimePublishValidation(t *testing.T) {
	rt := runtime.New()
	h := NewRuntimeHandler(rt.Bus, rt.Inspector)
	mux := http.NewServeMux()
	h.Register(mux)

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/runtime/publish", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/runtime/publish", strings.NewReader("{"))
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestRuntimeEventsWS(t *testing.T) {
	rt := runtime.New()
	h := NewRuntimeHandler(rt.Bus, rt.Inspector)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	parsed, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	wsURL := "ws://" + parsed.Host + "/runtime/events/ws?name=live2d.reload"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("open websocket: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(1200 * time.Millisecond))

	publishBody, _ := json.Marshal(map[string]any{
		"name":    "live2d.reload",
		"payload": map[string]any{"source": "test"},
	})
	postResp, err := http.Post(ts.URL+"/runtime/publish", "application/json", bytes.NewReader(publishBody))
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	postResp.Body.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket: %v", err)
	}

	var evt bus.Event
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal websocket payload: %v", err)
	}
	if evt.Name != "live2d.reload" {
		t.Fatalf("unexpected event name: %s", evt.Name)
	}
}

func TestRuntimeDiagnostics(t *testing.T) {
	rt := runtime.New()
	h := NewRuntimeHandler(rt.Bus, rt.Inspector)
	mux := http.NewServeMux()
	h.Register(mux)

	rt.Bus.Publish(bus.Event{Name: "audio.vad.start", SessionID: "s1", TraceID: "t1"})
	rt.Bus.Publish(bus.Event{Name: "asr.partial", SessionID: "s1", TraceID: "t1", Payload: "你好"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/runtime/diagnostics?limit=1", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var snapshot runtime.DiagnosticSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("unmarshal diagnostics: %v", err)
	}
	if snapshot.State == "" || snapshot.LastEvent == "" {
		t.Fatalf("expected populated snapshot: %#v", snapshot)
	}
	if len(snapshot.Events) != 1 {
		t.Fatalf("expected limited events, got %d", len(snapshot.Events))
	}
}
