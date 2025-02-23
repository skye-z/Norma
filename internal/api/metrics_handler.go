package api

import (
	"net/http"

	"norma/internal/metrics"
)

type MetricsHandler struct{ tracker *metrics.Tracker }

func NewMetricsHandler(tracker *metrics.Tracker) *MetricsHandler {
	return &MetricsHandler{tracker: tracker}
}

func (h *MetricsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/metrics/summary", h.handleSummary)
}

func (h *MetricsHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.tracker == nil {
		writeErr(w, http.StatusNotImplemented, "metrics unavailable")
		return
	}
	writeJSON(w, http.StatusOK, h.tracker.Snapshot())
}
