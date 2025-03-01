package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"norma/internal/config"
	"norma/internal/errs"
	"norma/internal/live2d"
	"norma/internal/providers"
)

type SetupHandler struct {
	configPath string
	modelDir   string
}

type SetupWizardReport struct {
	Ready       bool                        `json:"ready"`
	Provider    map[string]providers.Health `json:"provider"`
	Model       live2d.ValidationResult     `json:"model"`
	Checks      []SetupCheck                `json:"checks"`
	Suggestions []string                    `json:"suggestions"`
}

type SetupCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint"`
}

func NewSetupHandler(configPath, modelDir string) *SetupHandler {
	return &SetupHandler{configPath: configPath, modelDir: modelDir}
}

func (h *SetupHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/setup/wizard", h.handleWizard)
}

func (h *SetupHandler) handleWizard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	mgr := providers.NewManager(cfg.Providers.Root)
	health := providers.BuildHealth(mgr)
	modelDir := strings.TrimSpace(h.modelDir)
	if modelDir == "" {
		modelDir = filepath.ToSlash("model")
	}
	model := live2d.ValidateModelDir(modelDir)
	checks := []SetupCheck{
		buildSetupCheck("Wake/ASR/TTS Provider", summarizeProviderReady(health), providerDetail(health), "先在设置页安装缺失的 Provider，再点击“应用受管 Provider”。"),
		buildSetupCheck("Live2D 模型", statusFromBool(model.OK()), modelDetail(model), "如果模型缺失，先补齐 .model3.json/.can3 与 .cmo3。"),
		buildSetupCheck("配置文件", statusFromBool(fileExists(h.configPath)), absOrRaw(h.configPath), "首启失败时先检查 configs/config.yaml 是否存在。"),
		buildSetupCheck("数据目录", statusFromBool(dirExists(filepath.Dir(cfg.Database.Path))), filepath.Dir(cfg.Database.Path), "若目录不存在，先创建 data 目录。"),
		buildSetupCheck("权限提示", "partial", "Windows 桌面悬浮与通知通常不需额外权限，但麦克风权限需在系统设置中允许。", "若无法录音，请在系统隐私设置中开启麦克风访问。"),
	}
	suggestions := buildSuggestions(health, model)
	ready := model.OK() && health["asr"].Status == "ready" && health["tts"].Status != "missing"
	writeJSON(w, http.StatusOK, SetupWizardReport{Ready: ready, Provider: health, Model: model, Checks: checks, Suggestions: suggestions})
}

func buildSetupCheck(name, status, detail, hint string) SetupCheck {
	return SetupCheck{Name: name, Status: status, Detail: detail, Hint: hint}
}

func summarizeProviderReady(health map[string]providers.Health) string {
	for _, kind := range []string{"wake", "asr", "tts"} {
		if health[kind].Status == "missing" {
			return "missing"
		}
		if health[kind].Status == "partial" {
			return "partial"
		}
	}
	return "ready"
}

func providerDetail(health map[string]providers.Health) string {
	parts := make([]string, 0, 3)
	for _, kind := range []string{"wake", "asr", "tts"} {
		parts = append(parts, kind+": "+health[kind].Status)
	}
	return strings.Join(parts, " / ")
}

func modelDetail(model live2d.ValidationResult) string {
	if model.OK() {
		return model.ModelDir
	}
	return strings.Join(model.Issues, "；")
}

func statusFromBool(ok bool) string {
	if ok {
		return "ready"
	}
	return "missing"
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func absOrRaw(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func buildSuggestions(health map[string]providers.Health, model live2d.ValidationResult) []string {
	suggestions := make([]string, 0, 6)
	if health["asr"].Status != "ready" {
		suggestions = append(suggestions, "补齐 whisper.cpp 二进制和 ggml 模型。")
	}
	if health["tts"].Status != "ready" {
		suggestions = append(suggestions, "补齐 Piper 二进制、.onnx 和 .onnx.json。")
	}
	if health["wake"].Status != "ready" {
		suggestions = append(suggestions, "补齐 sherpa-onnx keyword spotter、模型文件和 keywords.txt。")
	}
	if !model.OK() {
		suggestions = append(suggestions, "Live2D 模型目录未通过校验，先修复模型资源。")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "首启检查通过，可以进入桌面伴侣模式。")
	}
	return suggestions
}
