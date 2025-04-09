package api

import (
	"net/http"

	config "norma/backend/utils"
)

type MetricsHandler struct{ tracker *config.Tracker }

func NewMetricsHandler(tracker *config.Tracker) *MetricsHandler {
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
