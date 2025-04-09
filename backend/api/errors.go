package api

import (
	"encoding/json"
	"net/http"

	errs "norma/backend/utils"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	apiErr := errs.New("api_error", msg, code, errs.SeverityUser, code >= 500, code < 500)
	writeAPIError(w, apiErr)
}

func writeAPIError(w http.ResponseWriter, apiErr *errs.Error) {
	if apiErr == nil {
		apiErr = errs.Internal("unknown_error", "unexpected error")
	}
	writeJSON(w, apiErr.Status, map[string]any{"error": apiErr})
}
