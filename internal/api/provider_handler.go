package api

import (
	"archive/tar"
	"compress/bzip2"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"norma/internal/config"
	"norma/internal/errs"
	"norma/internal/providers"
	"norma/pkg/layout"
)

type ProviderHandler struct {
	configPath string
	jobsMu     sync.Mutex
	jobs       map[string]*providerDownloadJob
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

type providerDiscoverItem struct {
	Key       string   `json:"key"`
	Name      string   `json:"name"`
	Installed bool     `json:"installed"`
	Paths     []string `json:"paths"`
	Hint      string   `json:"hint"`
}

type providerDownloadJob struct {
	ID         string  `json:"id"`
	Module     string  `json:"module"`
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	TargetDir  string  `json:"target_dir"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Error      string  `json:"error,omitempty"`
	UpdatedAt  string  `json:"updated_at"`
}

type providerDownloadStartRequest struct {
	Module string `json:"module"`
}

type progressWriter struct {
	total      int64
	written    int64
	onProgress func(written int64, total int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.written += int64(n)
	if w.onProgress != nil {
		w.onProgress(w.written, w.total)
	}
	return n, nil
}

func NewProviderHandler(configPath string) *ProviderHandler {
	return &ProviderHandler{configPath: configPath, jobs: make(map[string]*providerDownloadJob)}
}

func (h *ProviderHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/providers/status", h.handleStatus)
	mux.HandleFunc("/providers/init", h.handleInit)
	mux.HandleFunc("/providers/catalog", h.handleCatalog)
	mux.HandleFunc("/providers/install", h.handleInstall)
	mux.HandleFunc("/providers/uninstall", h.handleUninstall)
	mux.HandleFunc("/providers/apply-managed", h.handleApplyManaged)
	mux.HandleFunc("/providers/export-readiness", h.handleExportReadiness)
	mux.HandleFunc("/providers/open-readiness-dir", h.handleOpenReadinessDir)
	mux.HandleFunc("/providers/discover", h.handleDiscover)
	mux.HandleFunc("/providers/model-download/start", h.handleModelDownloadStart)
	mux.HandleFunc("/providers/model-download/status", h.handleModelDownloadStatus)
	mux.HandleFunc("/providers/model-download/open-dir", h.handleModelDownloadOpenDir)
}

func (h *ProviderHandler) handleModelDownloadStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	var req providerDownloadStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, errs.Invalid("invalid download start request"))
		return
	}
	module := strings.TrimSpace(req.Module)
	itemID := ""
	kind := ""
	switch module {
	case "asr_model":
		itemID = "asr-whisper-model-base"
		kind = "asr"
	case "wake_model":
		itemID = "wake-sherpa-kws-zh-en-model"
		kind = "wake"
	default:
		writeAPIError(w, errs.Invalid("unsupported module"))
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

	catalog, err := h.readCatalog()
	if err != nil {
		writeAPIError(w, errs.Internal("provider_catalog_load_failed", err.Error()))
		return
	}
	item := findCatalogItem(catalog, itemID)
	if item == nil {
		writeAPIError(w, errs.Invalid("catalog item not found: "+itemID))
		return
	}

	jobID := "mdl-" + strings.ReplaceAll(time.Now().Format("20060102-150405.000"), ".", "")
	targetDir := filepath.Join(mgr.Root(), kind, "current")
	job := &providerDownloadJob{
		ID:        jobID,
		Module:    module,
		Status:    "running",
		Message:   "准备下载",
		TargetDir: targetDir,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	h.jobsMu.Lock()
	h.jobs[jobID] = job
	h.jobsMu.Unlock()

	go h.runModelDownloadJob(jobID, item, targetDir)

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": job})
}

func (h *ProviderHandler) handleModelDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))
	if jobID == "" {
		writeAPIError(w, errs.Invalid("job_id required"))
		return
	}
	h.jobsMu.Lock()
	job, ok := h.jobs[jobID]
	h.jobsMu.Unlock()
	if !ok {
		writeAPIError(w, errs.Invalid("job not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": job})
}

func (h *ProviderHandler) handleModelDownloadOpenDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	var req providerDownloadStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, errs.Invalid("invalid open dir request"))
		return
	}
	module := strings.TrimSpace(req.Module)
	kind := ""
	switch module {
	case "asr_model":
		kind = "asr"
	case "wake_model":
		kind = "wake"
	default:
		writeAPIError(w, errs.Invalid("unsupported module"))
		return
	}

	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	mgr := providers.NewManager(cfg.Providers.Root)
	targetDir := filepath.Join(mgr.Root(), kind, "current")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		writeAPIError(w, errs.Internal("provider_open_dir_failed", err.Error()))
		return
	}
	if err := exec.Command("explorer", filepath.Clean(targetDir)).Start(); err != nil {
		writeAPIError(w, errs.Internal("provider_open_dir_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": targetDir})
}

func (h *ProviderHandler) readCatalog() (providerCatalog, error) {
	path := filepath.ToSlash("build/provider-catalog.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return providerCatalog{}, err
	}
	var out providerCatalog
	if err := json.Unmarshal(b, &out); err != nil {
		return providerCatalog{}, err
	}
	return out, nil
}

func findCatalogItem(catalog providerCatalog, id string) map[string]any {
	for _, item := range catalog.Items {
		if strings.TrimSpace(toString(item["id"])) == strings.TrimSpace(id) {
			return item
		}
	}
	return nil
}

func (h *ProviderHandler) updateJob(jobID string, mutate func(*providerDownloadJob)) {
	h.jobsMu.Lock()
	defer h.jobsMu.Unlock()
	job, ok := h.jobs[jobID]
	if !ok {
		return
	}
	mutate(job)
	job.UpdatedAt = time.Now().Format(time.RFC3339)
}

func (h *ProviderHandler) runModelDownloadJob(jobID string, item map[string]any, targetDir string) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "创建目录失败"
		})
		return
	}

	source := strings.TrimSpace(toString(item["source"]))
	archiveType := strings.TrimSpace(strings.ToLower(toString(item["archive_type"])))
	targetName := strings.TrimSpace(toString(item["target_name"]))
	if source == "" {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = "catalog source is empty"
			job.Message = "下载地址为空"
		})
		return
	}

	tmpDir, err := os.MkdirTemp("", "norma-model-download-")
	if err != nil {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "创建临时目录失败"
		})
		return
	}
	defer os.RemoveAll(tmpDir)

	fileName := filepath.Base(strings.Split(source, "?")[0])
	archivePath := filepath.Join(tmpDir, fileName)

	h.updateJob(jobID, func(job *providerDownloadJob) {
		job.Status = "running"
		job.Message = "开始下载"
	})

	resp, err := http.Get(source)
	if err != nil {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "下载失败"
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = "http status " + toString(resp.StatusCode)
			job.Message = "下载失败"
		})
		return
	}

	out, err := os.Create(archivePath)
	if err != nil {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "创建文件失败"
		})
		return
	}

	pw := &progressWriter{total: resp.ContentLength, onProgress: func(written int64, total int64) {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Downloaded = written
			job.Total = total
			if total > 0 {
				job.Percent = float64(written) * 100.0 / float64(total)
			}
			job.Message = "下载中"
		})
	}}

	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	_ = out.Close()
	if err != nil {
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "下载中断"
		})
		return
	}

	expected := strings.TrimSpace(toString(item["sha256"]))
	if expected != "" {
		actual, aerr := fileSHA256(archivePath)
		if aerr != nil || !strings.EqualFold(actual, expected) {
			h.updateJob(jobID, func(job *providerDownloadJob) {
				job.Status = "failed"
				job.Error = "sha256 mismatch"
				job.Message = "校验失败"
			})
			return
		}
	}

	h.updateJob(jobID, func(job *providerDownloadJob) { job.Message = "安装中" })

	switch archiveType {
	case "file", "":
		if targetName == "" {
			targetName = fileName
		}
		if err := copyFile(archivePath, filepath.Join(targetDir, targetName)); err != nil {
			h.updateJob(jobID, func(job *providerDownloadJob) {
				job.Status = "failed"
				job.Error = err.Error()
				job.Message = "安装失败"
			})
			return
		}
	case "tar.bz2":
		if err := extractTarBz2(archivePath, targetDir); err != nil {
			h.updateJob(jobID, func(job *providerDownloadJob) {
				job.Status = "failed"
				job.Error = err.Error()
				job.Message = "解压失败"
			})
			return
		}
	default:
		h.updateJob(jobID, func(job *providerDownloadJob) {
			job.Status = "failed"
			job.Error = "unsupported archive_type: " + archiveType
			job.Message = "安装失败"
		})
		return
	}

	if files, ok := item["managed_files"].([]any); ok {
		for _, v := range files {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			tn := strings.TrimSpace(toString(m["target_name"]))
			if tn == "" {
				continue
			}
			content := toString(m["content"])
			_ = os.WriteFile(filepath.Join(targetDir, tn), []byte(content), 0o644)
		}
	}

	h.updateJob(jobID, func(job *providerDownloadJob) {
		job.Status = "done"
		job.Message = "下载并安装完成"
		job.Percent = 100
	})
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		return ""
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func extractTarBz2(archivePath, targetDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	bzr := bzip2.NewReader(f)
	tr := tar.NewReader(bzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.Contains(name, "..") {
			continue
		}
		outPath := filepath.Join(targetDir, name)
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		_ = out.Close()
	}
	return nil
}

func (h *ProviderHandler) handleDiscover(w http.ResponseWriter, r *http.Request) {
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

	findAll := func(kind string, names ...string) []string {
		set := map[string]struct{}{}
		out := make([]string, 0)
		for _, name := range names {
			if p := mgr.FindFirst(kind, name); p != "" {
				if _, ok := set[p]; !ok {
					set[p] = struct{}{}
					out = append(out, p)
				}
			}
			if p, lookErr := exec.LookPath(name); lookErr == nil && p != "" {
				if _, ok := set[p]; !ok {
					set[p] = struct{}{}
					out = append(out, p)
				}
			}
		}
		return out
	}

	items := []providerDiscoverItem{
		{
			Key:       "ollama",
			Name:      "Ollama",
			Paths:     findAll("llm", "ollama.exe", "ollama"),
			Hint:      "用于 LLM/Embedding",
			Installed: false,
		},
		{
			Key:       "sherpa-onnx-tts",
			Name:      "Sherpa ONNX TTS",
			Paths:     discoverKokoroAssets(mgr),
			Hint:      "用于 TTS (Kokoro)",
			Installed: false,
		},
		{
			Key:       "whispercpp",
			Name:      "Whisper.cpp",
			Paths:     findAll("asr", "whisper-cli.exe", "whisper-cli", "main.exe", "main"),
			Hint:      "用于 ASR",
			Installed: false,
		},
		{
			Key:       "sherpa-onnx",
			Name:      "Sherpa ONNX",
			Paths:     discoverWakeAssets(mgr),
			Hint:      "用于唤醒词",
			Installed: false,
		},
	}

	for i := range items {
		items[i].Installed = len(items[i].Paths) > 0
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"root":  mgr.Root(),
		"items": items,
	})
}

func discoverWakeAssets(mgr *providers.Manager) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, 5)
	appendOnce := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		if _, ok := set[path]; ok {
			return
		}
		set[path] = struct{}{}
		out = append(out, path)
	}
	appendOnce(mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx"))
	appendOnce(mgr.FindGlob("wake", "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx"))
	appendOnce(mgr.FindGlob("wake", "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx"))
	appendOnce(mgr.FindFirst("wake", "tokens.txt"))
	appendOnce(mgr.FindFirst("wake", "keywords.txt", "test_keywords.txt"))
	return out
}

func discoverKokoroAssets(mgr *providers.Manager) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, 6)
	appendOnce := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		if _, ok := set[path]; ok {
			return
		}
		set[path] = struct{}{}
		out = append(out, path)
	}
	appendOnce(mgr.FindFirst("tts", "model.onnx"))
	appendOnce(mgr.FindFirst("tts", "voices.bin"))
	appendOnce(mgr.FindFirst("tts", "tokens.txt"))
	appendOnce(mgr.FindGlob("tts", "espeak-ng-data"))
	appendOnce(mgr.FindFirst("tts", "lexicon-us-en.txt"))
	appendOnce(mgr.FindFirst("tts", "lexicon-zh.txt"))
	return out
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
	catalogPath := filepath.ToSlash("build/provider-catalog.json")
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
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", "scripts/provider-install.ps1", "-CatalogPath", "build/provider-catalog.json", "-ProviderId", req.ID, "-ProvidersRoot", cfg.Providers.Root)
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
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", "scripts/provider-install.ps1", "-CatalogPath", "build/provider-catalog.json", "-ProviderId", req.ID, "-ProvidersRoot", cfg.Providers.Root, "-Uninstall")
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
	cfg.Audio.EnableWakeWord = true
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
	cfg.TTS.Provider = "sherpa-onnx"
	if cfg.TTS.RequestTimeoutMs <= 0 {
		cfg.TTS.RequestTimeoutMs = 30000
	}
	cfg.TTS.SentenceFirstEnabled = true
	cfg.RAG.EmbedProvider = "ollama"
	if cfg.Perf.DegradedEFSearch <= 0 {
		cfg.Perf.DegradedEFSearch = 24
	}
	if cfg.Perf.DegradedRetrieverExtra <= 0 {
		cfg.Perf.DegradedRetrieverExtra = 1
	}
	if cfg.Perf.DegradedRetrieverFuse <= 0 {
		cfg.Perf.DegradedRetrieverFuse = 2
	}
	if strings.TrimSpace(cfg.Perf.DegradedTTSVoice) == "" {
		cfg.Perf.DegradedTTSVoice = "zh_CN-huayan-x_low"
	}
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
	if p := mgr.FindFirst("tts", "model.onnx"); p != "" {
		cfg.TTS.ModelPath = p
	}
	if strings.TrimSpace(cfg.TTS.Voice) == "" || strings.HasPrefix(strings.TrimSpace(cfg.TTS.Voice), "zh_CN-") {
		cfg.TTS.Voice = "zf_xiaobei"
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
	manifest, err := mgr.LoadManifest()
	if err != nil {
		writeAPIError(w, errs.Internal("provider_manifest_load_failed", err.Error()))
		return
	}
	stack := map[string]string{
		"wake":      cfg.Audio.WakeProvider,
		"asr":       cfg.ASR.Provider,
		"tts":       cfg.TTS.Provider,
		"embedding": cfg.RAG.EmbedProvider,
		"llm":       cfg.LLM.Provider,
	}
	report := map[string]any{
		"exported_at": time.Now().Format(time.RFC3339),
		"root":        mgr.Root(),
		"stack":       stack,
		"health":      providers.BuildHealth(mgr),
		"manifest":    manifest,
	}
	outDir := layout.Join("", filepath.ToSlash(layout.DiagnosticsDir+"/provider-readiness"))
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

func (h *ProviderHandler) handleOpenReadinessDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	outDir := layout.Join("", filepath.ToSlash(layout.DiagnosticsDir+"/provider-readiness"))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		writeAPIError(w, errs.Internal("provider_open_dir_failed", err.Error()))
		return
	}
	if err := exec.Command("explorer", filepath.Clean(outDir)).Start(); err != nil {
		writeAPIError(w, errs.Internal("provider_open_dir_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": outDir})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
