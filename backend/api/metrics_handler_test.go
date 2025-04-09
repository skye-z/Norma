package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"norma/backend/utils"
)

func TestMetricsSummary(t *testing.T) {
	tracker := metrics.NewTracker()
	tracker.RecordValue("audio.vad.start", 1)
	h := NewMetricsHandler(tracker)
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics/summary", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var snap metrics.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(snap.Counters) == 0 {
		t.Fatalf("expected counters")
	}
}
