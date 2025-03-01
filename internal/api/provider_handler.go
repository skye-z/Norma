package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"norma/internal/config"
	"norma/internal/errs"
	"norma/internal/providers"
)

type ProviderHandler struct {
	configPath string
}

type providerStatusResponse struct {
	Root      string                      `json:"root"`
	Providers map[string][]string         `json:"providers"`
	Manifest  providers.Manifest          `json:"manifest"`
	Stack     map[string]string           `json:"stack"`
	Health    map[string]providers.Health `json:"health"`
}

type providerCatalog struct {
	Version int              `json:"version"`
	Items   []map[string]any `json:"items"`
}

type providerInstallRequest struct {
	ID string `json:"id"`
}

func NewProviderHandler(configPath string) *ProviderHandler {
	return &ProviderHandler{configPath: configPath}
}

func (h *ProviderHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/providers/status", h.handleStatus)
	mux.HandleFunc("/providers/init", h.handleInit)
	mux.HandleFunc("/providers/catalog", h.handleCatalog)
	mux.HandleFunc("/providers/install", h.handleInstall)
	mux.HandleFunc("/providers/uninstall", h.handleUninstall)
	mux.HandleFunc("/providers/apply-managed", h.handleApplyManaged)
	mux.HandleFunc("/providers/export-readiness", h.handleExportReadiness)
}

func (h *ProviderHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	mgr := providers.NewManager(cfg.Providers.Root)
	manifest, err := mgr.LoadManifest()
	if err != nil {
		writeAPIError(w, errs.Internal("provider_manifest_load_failed", err.Error()))
		return
	}
	items := make(map[string][]string, len(providers.KnownKinds))
	for _, kind := range providers.KnownKinds {
		files, ferr := mgr.CurrentFiles(kind)
		if ferr != nil {
			writeAPIError(w, errs.Internal("provider_status_failed", ferr.Error()))
			return
		}
		items[kind] = files
	}
	writeJSON(w, http.StatusOK, providerStatusResponse{
		Root:      mgr.Root(),
		Providers: items,
		Manifest:  manifest,
		Stack: map[string]string{
			"wake":      cfg.Audio.WakeProvider,
			"asr":       cfg.ASR.Provider,
			"tts":       cfg.TTS.Provider,
			"embedding": cfg.RAG.EmbedProvider,
			"llm":       cfg.LLM.Provider,
		},
		Health: providers.BuildHealth(mgr),
	})
}

func (h *ProviderHandler) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	mgr := providers.NewManager(cfg.Providers.Root)
	if err := mgr.EnsureLayout(); err != nil {
		writeAPIError(w, errs.Internal("provider_layout_init_failed", err.Error()))
		return
	}
	manifest, err := mgr.LoadManifest()
	if err != nil {
		writeAPIError(w, errs.Internal("provider_manifest_load_failed", err.Error()))
		return
	}
	if err := mgr.SaveManifest(manifest); err != nil {
		writeAPIError(w, errs.Internal("provider_manifest_save_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "root": mgr.Root()})
}

func (h *ProviderHandler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	catalogPath := filepath.ToSlash("configs/provider-catalog.json")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		writeAPIError(w, errs.Internal("provider_catalog_read_failed", err.Error()))
		return
	}
	var out providerCatalog
	if err := json.Unmarshal(data, &out); err != nil {
		writeAPIError(w, errs.Internal("provider_catalog_decode_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *ProviderHandler) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	var req providerInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeAPIError(w, errs.Invalid("invalid install request"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", "scripts/provider-install.ps1", "-CatalogPath", "configs/provider-catalog.json", "-ProviderId", req.ID, "-ProvidersRoot", cfg.Providers.Root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeAPIError(w, errs.Internal("provider_install_failed", string(out)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": string(out)})
}

func (h *ProviderHandler) handleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	var req providerInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeAPIError(w, errs.Invalid("invalid uninstall request"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", "scripts/provider-install.ps1", "-CatalogPath", "configs/provider-catalog.json", "-ProviderId", req.ID, "-ProvidersRoot", cfg.Providers.Root, "-Uninstall")
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeAPIError(w, errs.Internal("provider_uninstall_failed", string(out)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": string(out)})
}

func (h *ProviderHandler) handleApplyManaged(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	mgr := providers.NewManager(cfg.Providers.Root)
	if err := mgr.EnsureLayout(); err != nil {
		writeAPIError(w, errs.Internal("provider_layout_init_failed", err.Error()))
		return
	}

	cfg.Audio.WakeMode = "wake_word_vad"
	cfg.Audio.WakeProvider = "sherpa-onnx"
	cfg.Audio.WakeKeyword = firstNonEmpty(cfg.Audio.WakeKeyword, "诺玛")
	if cfg.Audio.WakeWindowMs <= 0 {
		cfg.Audio.WakeWindowMs = 2000
	}
	if cfg.Audio.WakeDetectIntervalMs <= 0 {
		cfg.Audio.WakeDetectIntervalMs = 400
	}
	cfg.ASR.Provider = "whispercpp"
	cfg.ASR.Language = firstNonEmpty(cfg.ASR.Language, "zh")
	if cfg.ASR.Threads <= 0 {
		cfg.ASR.Threads = 4
	}
	if cfg.ASR.RequestTimeoutMs <= 0 {
		cfg.ASR.RequestTimeoutMs = 10000
	}
	cfg.TTS.Provider = "piper"
	if cfg.TTS.RequestTimeoutMs <= 0 {
		cfg.TTS.RequestTimeoutMs = 30000
	}
	cfg.TTS.SentenceFirstEnabled = true
	cfg.RAG.EmbedProvider = "ollama"
	if cfg.RAG.TopK <= 0 {
		cfg.RAG.TopK = 6
	}
	if strings.TrimSpace(cfg.RAG.EmbedModel) == "" {
		cfg.RAG.EmbedModel = "bge-m3:latest"
	}

	if p := mgr.FindFirst("asr", "whisper-cli.exe", "main.exe", "whisper-cli", "main"); p != "" {
		cfg.ASR.BinaryPath = p
	}
	if p := mgr.FindFirst("asr", "ggml-base.bin", "ggml-small.bin", "ggml-medium.bin", "ggml-large-v3.bin"); p != "" {
		cfg.ASR.ModelPath = p
	}
	if p := mgr.FindFirst("tts", "piper.exe", "piper"); p != "" {
		cfg.TTS.BinaryPath = p
	}
	if p := mgr.FindFirst("tts", "zh_CN-huayan-medium.onnx", "zh_CN-huayan-x_low.onnx", "model.onnx"); p != "" {
		cfg.TTS.ModelPath = p
		base := filepath.Base(p)
		cfg.TTS.Voice = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if p := mgr.FindFirst("wake", "sherpa-onnx-keyword-spotter.exe", "sherpa-onnx-keyword-spotter"); p != "" {
		cfg.Audio.WakeBinaryPath = p
	}
	if p := mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx"); p != "" {
		cfg.Audio.WakeModelPath = filepath.Dir(p)
	}
	if p := mgr.FindFirst("wake", "keywords.txt", "test_keywords.txt"); p != "" {
		cfg.Audio.WakeKeywordsFilePath = p
	}
	if p := mgr.FindFirst("llm", "ollama.exe", "ollama"); p != "" {
		_ = p
		if strings.TrimSpace(cfg.LLM.Provider) == "" {
			cfg.LLM.Provider = "ollama"
		}
		if strings.TrimSpace(cfg.LLM.Endpoint) == "" {
			cfg.LLM.Endpoint = "http://127.0.0.1:11434"
		}
		if strings.TrimSpace(cfg.RAG.EmbedEndpoint) == "" {
			cfg.RAG.EmbedEndpoint = cfg.LLM.Endpoint
		}
	}

	if err := config.Save(h.configPath, cfg); err != nil {
		writeAPIError(w, errs.Internal("config_save_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "config_path": h.configPath})
}

func (h *ProviderHandler) handleExportReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	mgr := providers.NewManager(cfg.Providers.Root)
	report := map[string]any{
		"exported_at": time.Now().Format(time.RFC3339),
		"root":        mgr.Root(),
		"health":      providers.BuildHealth(mgr),
	}
	outDir := filepath.Join("diagnostics", "provider-readiness")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		writeAPIError(w, errs.Internal("provider_export_failed", err.Error()))
		return
	}
	path := filepath.Join(outDir, "readiness-"+time.Now().Format("20060102-150405")+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		writeAPIError(w, errs.Internal("provider_export_failed", err.Error()))
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		writeAPIError(w, errs.Internal("provider_export_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
