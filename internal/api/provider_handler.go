package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"norma/internal/config"
	"norma/internal/errs"
	"norma/internal/providers"
)

type ProviderHandler struct {
	configPath string
}

type providerStatusResponse struct {
	Root      string              `json:"root"`
	Providers map[string][]string `json:"providers"`
	Manifest  providers.Manifest  `json:"manifest"`
	Stack     map[string]string   `json:"stack"`
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
