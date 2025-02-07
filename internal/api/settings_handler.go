package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"norma/internal/config"
	"norma/internal/runtime/bus"
)

type SettingsHandler struct {
	configPath string
	bus        *bus.Bus
}

func NewSettingsHandler(configPath string, b *bus.Bus) *SettingsHandler {
	return &SettingsHandler{configPath: configPath, bus: b}
}

func (h *SettingsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/settings/runtime", h.handleRuntime)
}

func (h *SettingsHandler) handleRuntime(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleRuntimeGet(w)
	case http.MethodPut:
		h.handleRuntimePut(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *SettingsHandler) handleRuntimeGet(w http.ResponseWriter) {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *SettingsHandler) handleRuntimePut(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(cfg.LLM.Provider) == "" {
		writeErr(w, http.StatusBadRequest, "llm.provider required")
		return
	}
	if strings.TrimSpace(cfg.LLM.Endpoint) == "" {
		writeErr(w, http.StatusBadRequest, "llm.endpoint required")
		return
	}
	if cfg.LLM.RequestTimeoutMs <= 0 {
		cfg.LLM.RequestTimeoutMs = 45000
	}
	if cfg.ASR.RequestTimeoutMs <= 0 {
		cfg.ASR.RequestTimeoutMs = 10000
	}
	if cfg.TTS.RequestTimeoutMs <= 0 {
		cfg.TTS.RequestTimeoutMs = 30000
	}
	if cfg.RAG.TopK <= 0 {
		cfg.RAG.TopK = 6
	}
	if cfg.Memory.STMTokenBudget <= 0 {
		cfg.Memory.STMTokenBudget = 2048
	}
	if cfg.Perf.LLMFirstTokenThresholdMs <= 0 {
		cfg.Perf.LLMFirstTokenThresholdMs = 1200
	}
	if cfg.Perf.DegradedRAGTopK <= 0 {
		cfg.Perf.DegradedRAGTopK = 4
	}

	if err := config.Save(h.configPath, &cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.bus != nil {
		h.bus.Publish(bus.Event{Name: "runtime.config.updated", Payload: map[string]any{"path": h.configPath}})
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
