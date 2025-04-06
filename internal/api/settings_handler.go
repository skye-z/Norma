package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"norma/internal/bootstrap"
	"norma/internal/config"
	"norma/internal/errs"
	"norma/internal/providers"
	"norma/internal/runtime/bus"
	"norma/pkg/layout"
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
	if normalizeTTSToSherpa(cfg) {
		if err := config.Save(h.configPath, cfg); err != nil {
			writeAPIError(w, errs.Internal("config_save_failed", err.Error()))
			return
		}
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *SettingsHandler) handleRuntimePut(w http.ResponseWriter, r *http.Request) {
	previous, _ := config.Load(h.configPath)
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeAPIError(w, errs.Invalid("invalid json"))
		return
	}
	_ = normalizeTTSToSherpa(&cfg)
	if strings.TrimSpace(cfg.LLM.Provider) == "" {
		writeAPIError(w, errs.Invalid("llm.provider required"))
		return
	}
	cfg.App.ModelName = strings.TrimSpace(cfg.App.ModelName)
	if strings.TrimSpace(cfg.LLM.Endpoint) == "" {
		writeAPIError(w, errs.Invalid("llm.endpoint required"))
		return
	}
	if cfg.LLM.RequestTimeoutMs <= 0 {
		cfg.LLM.RequestTimeoutMs = 45000
	}
	cfg.LLM.APIKey = strings.TrimSpace(cfg.LLM.APIKey)
	if strings.TrimSpace(cfg.LLM.Persona) == "" {
		cfg.LLM.Persona = config.DefaultLLMPersona
	}
	if strings.TrimSpace(cfg.Audio.WakeProvider) == "" {
		cfg.Audio.WakeProvider = "sherpa-onnx"
	}
	if !cfg.Audio.EnableWakeWord {
		cfg.Audio.WakeMode = "vad"
	} else if strings.TrimSpace(cfg.Audio.WakeMode) == "" || strings.EqualFold(strings.TrimSpace(cfg.Audio.WakeMode), "vad") {
		cfg.Audio.WakeMode = "wake_word_vad"
	}
	if strings.TrimSpace(cfg.Audio.MicDeviceID) == "" {
		cfg.Audio.MicDeviceID = "default"
	}
	if cfg.Audio.MicSampleRate <= 0 {
		cfg.Audio.MicSampleRate = 16000
	}
	if cfg.Audio.MicChannelCount <= 0 {
		cfg.Audio.MicChannelCount = 1
	}
	if strings.TrimSpace(cfg.Audio.WakeKeyword) == "" {
		cfg.Audio.WakeKeyword = "诺玛"
	}
	if cfg.Audio.WakeWindowMs <= 0 {
		cfg.Audio.WakeWindowMs = 2000
	}
	if cfg.Audio.WakeDetectIntervalMs <= 0 {
		cfg.Audio.WakeDetectIntervalMs = 400
	}
	if cfg.ASR.RequestTimeoutMs <= 0 {
		cfg.ASR.RequestTimeoutMs = 10000
	}
	if cfg.TTS.RequestTimeoutMs <= 0 {
		cfg.TTS.RequestTimeoutMs = 30000
	}
	if strings.TrimSpace(cfg.TTS.Provider) == "" {
		cfg.TTS.Provider = "sherpa-onnx"
	}
	if cfg.RAG.TopK <= 0 {
		cfg.RAG.TopK = 6
	}
	if strings.TrimSpace(cfg.RAG.EmbedProvider) == "" {
		cfg.RAG.EmbedProvider = "ollama"
	}
	if strings.TrimSpace(cfg.RAG.EmbedEndpoint) == "" {
		cfg.RAG.EmbedEndpoint = cfg.LLM.Endpoint
	}
	if cfg.Memory.STMTokenBudget <= 0 {
		cfg.Memory.STMTokenBudget = 2048
	}
	if strings.TrimSpace(cfg.Providers.Root) == "" {
		cfg.Providers.Root = filepath.ToSlash(layout.ProvidersDir)
	}
	if strings.TrimSpace(cfg.TTS.ModelPath) == "" {
		mgr := providers.NewManager(cfg.Providers.Root)
		if p := mgr.FindFirst("tts", "model.onnx"); p != "" {
			cfg.TTS.ModelPath = p
		}
	}
	if cfg.Security.PurgeProofDir == "" {
		cfg.Security.PurgeProofDir = filepath.ToSlash(layout.PurgeProofDir)
	}
	if len(cfg.Security.AllowedRemoteHosts) == 0 {
		cfg.Security.AllowedRemoteHosts = []string{"127.0.0.1", "localhost"}
	}
	if len(cfg.Security.ToolWhitelist) == 0 {
		cfg.Security.ToolWhitelist = []string{"open_local_file", "open_debug_panel", "export_diagnostics"}
	}

	for _, endpoint := range []string{cfg.LLM.Endpoint, cfg.ASR.Endpoint, cfg.ASR.StreamEndpoint, cfg.TTS.Endpoint, cfg.RAG.EmbedEndpoint} {
		if err := validateEndpointPolicy(endpoint, cfg.Security); err != nil {
			writeAPIError(w, err)
			return
		}
	}

	if err := syncWakeKeywordArtifacts(&cfg); err != nil {
		writeAPIError(w, errs.Internal("wake_keywords_sync_failed", err.Error()))
		return
	}
	if err := config.Save(h.configPath, &cfg); err != nil {
		writeAPIError(w, errs.Internal("config_save_failed", err.Error()))
		return
	}
	if h.bus != nil {
		h.bus.Publish(bus.Event{Name: "runtime.config.updated", Payload: map[string]any{"path": h.configPath}})
		publishWakeModeAnnouncement(h.bus, previous, &cfg, "settings")
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

func normalizeTTSToSherpa(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	changed := false
	if strings.TrimSpace(cfg.TTS.Provider) == "" || strings.EqualFold(strings.TrimSpace(cfg.TTS.Provider), "piper") {
		cfg.TTS.Provider = "sherpa-onnx"
		changed = true
	}
	if strings.HasPrefix(strings.TrimSpace(cfg.TTS.Voice), "zh_CN-") {
		cfg.TTS.Voice = "zf_xiaobei"
		changed = true
	}
	if strings.TrimSpace(cfg.TTS.Voice) == "" {
		cfg.TTS.Voice = "zf_xiaobei"
		changed = true
	}
	if strings.TrimSpace(cfg.Providers.Root) == "" {
		cfg.Providers.Root = filepath.ToSlash(layout.ProvidersDir)
		changed = true
	}
	if strings.TrimSpace(cfg.TTS.ModelPath) == "" {
		mgr := providers.NewManager(cfg.Providers.Root)
		if p := mgr.FindFirst("tts", "model.onnx"); p != "" {
			cfg.TTS.ModelPath = p
			changed = true
		}
	}
	return changed
}

func syncWakeKeywordArtifacts(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	mgr := providers.NewManager(cfg.Providers.Root)
	if err := mgr.EnsureLayout(); err != nil {
		return err
	}
	keywordsPath, err := bootstrap.EnsureWakeKeywordsForConfig(cfg, mgr)
	if err != nil {
		return err
	}
	if strings.TrimSpace(keywordsPath) == "" {
		keywordsPath = strings.TrimSpace(cfg.Audio.WakeKeywordsFilePath)
	}
	if strings.TrimSpace(keywordsPath) == "" {
		modelDir := strings.TrimSpace(cfg.Audio.WakeModelPath)
		if modelDir == "" {
			modelDir = filepath.Join(mgr.Root(), "wake", "current")
		}
		keywordsPath = filepath.Join(modelDir, "keywords.txt")
	}
	if err := os.MkdirAll(filepath.Dir(keywordsPath), 0o755); err != nil {
		return err
	}
	formatted, err := providers.FormatWakeKeyword(filepath.Dir(keywordsPath), cfg.Audio.WakeKeyword)
	if err != nil {
		return err
	}
	if err := os.WriteFile(keywordsPath, []byte(formatted), 0o644); err != nil {
		return err
	}
	cfg.Audio.WakeKeywordsFilePath = filepath.ToSlash(keywordsPath)
	return nil
}

func autoWakeModeEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.Audio.EnableWakeWord && strings.EqualFold(strings.TrimSpace(cfg.Audio.WakeMode), "wake_word_vad")
}

func publishWakeModeAnnouncement(b *bus.Bus, previous, current *config.Config, source string) {
	if b == nil || current == nil {
		return
	}
	prev := autoWakeModeEnabled(previous)
	next := autoWakeModeEnabled(current)
	if prev == next {
		return
	}
	text := "我先休息啦，有事按我头上的按钮"
	if next {
		text = "我一直都在，有事叫我哦"
	}
	b.Publish(bus.Event{Name: "assistant.say", SessionID: "system-wake-mode", TraceID: "system-wake-mode", Payload: map[string]any{"text": text, "interrupt": true, "source": source}})
}
