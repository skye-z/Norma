package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"norma/internal/memory"
)

type MemoryHandler struct {
	svc *memory.Service
}

func NewMemoryHandler(svc *memory.Service) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

func (h *MemoryHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/memory", h.handleMemory)
	mux.HandleFunc("/memory/", h.handleMemoryByID)
	mux.HandleFunc("/memory/governance/run", h.handleGovernanceRun)
	mux.HandleFunc("/memory/summary", h.handleSummary)
}

func (h *MemoryHandler) handleMemory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query().Get("q")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		includeDisabled := r.URL.Query().Get("include_disabled") == "1"
		if strings.TrimSpace(q) == "" {
			hits, err := h.svc.List(context.Background(), includeDisabled, limit)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, hits)
			return
		}
		hits, err := h.svc.Search(context.Background(), q, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, hits)
	case http.MethodPost:
		var req memory.AddInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		m, err := h.svc.AddDetailed(context.Background(), req)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if m == nil {
			writeErr(w, http.StatusBadRequest, "empty content")
			return
		}
		writeJSON(w, http.StatusCreated, m)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *MemoryHandler) handleMemoryByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/memory/")
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := h.svc.Delete(context.Background(), id); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
	case http.MethodPut:
		var req memory.UpdateInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		item, err := h.svc.Update(context.Background(), id, req)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *MemoryHandler) handleGovernanceRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	result, err := h.svc.GovernanceRun(context.Background())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type summaryReq struct {
	SessionID string        `json:"session_id"`
	Turns     []memory.Turn `json:"turns"`
}

func (h *MemoryHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req summaryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	result, err := h.svc.SummarizeTurn(context.Background(), req.SessionID, req.Turns)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}
