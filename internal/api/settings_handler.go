package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"norma/internal/config"
	"norma/internal/errs"
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
		writeAPIError(w, errs.Invalid("method not allowed"))
	}
}

func (h *SettingsHandler) handleRuntimeGet(w http.ResponseWriter) {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *SettingsHandler) handleRuntimePut(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeAPIError(w, errs.Invalid("invalid json"))
		return
	}
	if strings.TrimSpace(cfg.LLM.Provider) == "" {
		writeAPIError(w, errs.Invalid("llm.provider required"))
		return
	}
	if strings.TrimSpace(cfg.LLM.Endpoint) == "" {
		writeAPIError(w, errs.Invalid("llm.endpoint required"))
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
	if cfg.Perf.DegradedAudioChunkSize <= 0 {
		cfg.Perf.DegradedAudioChunkSize = 4096
	}
	if cfg.Perf.AlertLLMFirstTokenMs <= 0 {
		cfg.Perf.AlertLLMFirstTokenMs = 1500
	}
	if cfg.Perf.AlertTTSFirstChunkMs <= 0 {
		cfg.Perf.AlertTTSFirstChunkMs = 1200
	}
	if cfg.Security.PurgeProofDir == "" {
		cfg.Security.PurgeProofDir = filepath.ToSlash("diagnostics/purge-proofs")
	}
	if len(cfg.Security.AllowedRemoteHosts) == 0 {
		cfg.Security.AllowedRemoteHosts = []string{"127.0.0.1", "localhost"}
	}
	if len(cfg.Security.ToolWhitelist) == 0 {
		cfg.Security.ToolWhitelist = []string{"open_local_file", "open_debug_panel", "export_diagnostics"}
	}

	for _, endpoint := range []string{cfg.LLM.Endpoint, cfg.ASR.Endpoint, cfg.ASR.StreamEndpoint, cfg.TTS.Endpoint} {
		if err := validateEndpointPolicy(endpoint, cfg.Security); err != nil {
			writeAPIError(w, err)
			return
		}
	}

	if err := config.Save(h.configPath, &cfg); err != nil {
		writeAPIError(w, errs.Internal("config_save_failed", err.Error()))
		return
	}
	if h.bus != nil {
		h.bus.Publish(bus.Event{Name: "runtime.config.updated", Payload: map[string]any{"path": h.configPath}})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func validateEndpointPolicy(endpoint string, sec config.SecurityConfig) *errs.Error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" {
		return errs.Invalid("invalid endpoint: " + endpoint)
	}
	host := strings.ToLower(u.Hostname())
	if sec.AllowRemoteProviders {
		return nil
	}
	for _, allowed := range sec.AllowedRemoteHosts {
		if strings.EqualFold(strings.TrimSpace(allowed), host) {
			return nil
		}
	}
	return errs.Forbidden("remote providers are disabled by security policy")
}
