// Package api 提供 HTTP API
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"norma/backend/core/memory"
)

// MemoryHandler 记忆处理器
type MemoryHandler struct {
	svc *memory.Service
}

// NewMemoryHandler 创建记忆处理器
func NewMemoryHandler(svc *memory.Service) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

// Register 注册路由
func (h *MemoryHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/memory", h.handleMemory)
	mux.HandleFunc("/memory/", h.handleMemoryByID)
	mux.HandleFunc("/memory/governance/run", h.handleGovernanceRun)
	mux.HandleFunc("/memory/summary", h.handleSummary)
}

// handleMemory 处理记忆 CRUD
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

// handleMemoryByID 根据 ID 处理记忆
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

// handleGovernanceRun 运行记忆治理
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

// summaryReq 摘要请求
type summaryReq struct {
	SessionID string        `json:"session_id"`
	Turns     []memory.Turn `json:"turns"`
}

// handleSummary 处理摘要请求
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
