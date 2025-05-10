package api

import (
	"net/http"
	"time"
)

// 健康检查
func RegisterHealth(mux *http.ServeMux, startedAt time.Time) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"started_at": startedAt.UTC().Format(time.RFC3339),
		})
	})
}
