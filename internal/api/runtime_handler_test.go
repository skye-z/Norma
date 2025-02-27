package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"norma/internal/runtime"
	"norma/internal/runtime/bus"
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

func TestRuntimeEventsSSE(t *testing.T) {
	rt := runtime.New()
	h := NewRuntimeHandler(rt.Bus, rt.Inspector)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/runtime/events?name=live2d.reload", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open sse: %v", err)
	}
	defer resp.Body.Close()

	publishBody, _ := json.Marshal(map[string]any{
		"name":    "live2d.reload",
		"payload": map[string]any{"source": "test"},
	})
	postResp, err := http.Post(ts.URL+"/runtime/publish", "application/json", bytes.NewReader(publishBody))
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	postResp.Body.Close()

	line, err := readSSEDataLine(resp.Body, 1200*time.Millisecond)
	if err != nil {
		t.Fatalf("read sse: %v", err)
	}
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("unexpected sse line: %s", line)
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

func readSSEDataLine(bodyReader interface{ Read([]byte) (int, error) }, timeout time.Duration) (string, error) {
	reader := bufio.NewReader(bodyReader)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			return line, nil
		}
	}
	return "", context.DeadlineExceeded
}
