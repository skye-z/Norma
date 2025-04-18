// Package api 提供 HTTP API
package api

import (
	"net/http"

	config "norma/backend/utils"
)

// MetricsHandler 指标处理器
type MetricsHandler struct{ tracker *config.Tracker }

// NewMetricsHandler 创建指标处理器
func NewMetricsHandler(tracker *config.Tracker) *MetricsHandler {
	return &MetricsHandler{tracker: tracker}
}

// Register 注册路由
func (h *MetricsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/metrics/summary", h.handleSummary)
}

// handleSummary 获取指标摘要
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
