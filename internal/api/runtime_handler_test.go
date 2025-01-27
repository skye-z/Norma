package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"norma/internal/runtime/bus"
)

func TestRuntimePublish(t *testing.T) {
	b := bus.New()
	h := NewRuntimeHandler(b)
	mux := http.NewServeMux()
	h.Register(mux)

	got := false
	_ = b.Subscribe("demo", func(evt bus.Event) {
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
