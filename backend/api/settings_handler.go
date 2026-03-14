// Package api 提供 HTTP API 处理器
// 包含对话、健康检查、设置等核心功能的 HTTP 端点
package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"norma/backend/core/bootstrap"
	"norma/backend/core/providers"
	"norma/backend/core/runtime/bus"
	"norma/backend/utils"
)

// SettingsHandler 处理运行时设置相关请求
// 支持获取和更新配置信息
type SettingsHandler struct {
	configPath string // 配置文件路径
	bus        *bus.Bus
}

// NewSettingsHandler 创建新的设置处理器实例
// 参数: configPath - 配置文件路径, b - 事件总线
// 返回: SettingsHandler 指针
func NewSettingsHandler(configPath string, b *bus.Bus) *SettingsHandler {
	return &SettingsHandler{configPath: configPath, bus: b}
}

// Register 注册设置相关的 HTTP 路由
// 添加 /settings/runtime 路由用于运行时配置管理
func (h *SettingsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/settings/runtime", h.handleRuntime)
}

// handleRuntime 处理运行时配置的 GET 和 PUT 请求
// 根据 HTTP 方法分发到对应的处理函数
func (h *SettingsHandler) handleRuntime(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleRuntimeGet(w)
	case http.MethodPut:
		h.handleRuntimePut(w, r)
	default:
		writeAPIError(w, config.Invalid("method not allowed"))
	}
}

// handleRuntimeGet 处理获取运行时配置的请求
// 加载配置文件并返回当前配置，支持自动规范化 TTS 配置
func (h *SettingsHandler) handleRuntimeGet(w http.ResponseWriter) {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, config.Internal("config_load_failed", err.Error()))
		return
	}
	// 自动将 TTS 配置规范化为 sherpa 格式
	if normalizeTTSToSherpa(cfg) {
		if err := config.Save(h.configPath, cfg); err != nil {
			writeAPIError(w, config.Internal("config_save_failed", err.Error()))
			return
		}
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleRuntimePut 处理更新运行时配置的请求
// 验证配置合法性并保存到文件，发布配置更新事件
func (h *SettingsHandler) handleRuntimePut(w http.ResponseWriter, r *http.Request) {
	previous, _ := config.Load(h.configPath)
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeAPIError(w, config.Invalid("invalid json"))
		return
	}
	// 规范化 TTS 配置为 sherpa 格式
	_ = normalizeTTSToSherpa(&cfg)

	// 验证必填字段: LLM Provider
	if strings.TrimSpace(cfg.LLM.Provider) == "" {
		writeAPIError(w, config.Invalid("llm.provider required"))
		return
	}
	// 规范化模型名称
	cfg.App.ModelName = strings.TrimSpace(cfg.App.ModelName)

	// 验证必填字段: LLM Endpoint
	if strings.TrimSpace(cfg.LLM.Endpoint) == "" {
		writeAPIError(w, config.Invalid("llm.endpoint required"))
		return
	}
	// 设置默认麦克风设备
	if strings.TrimSpace(cfg.Audio.MicDeviceID) == "" {
		cfg.Audio.MicDeviceID = "default"
	}
	// 设置默认采样率 (16kHz)
	if cfg.Audio.SampleRate <= 0 {
		cfg.Audio.SampleRate = 16000
	}
	// 设置默认声道数 (单声道)
	if cfg.Audio.Channels <= 0 {
		cfg.Audio.Channels = 1
	}
	// 设置默认唤醒词
	if strings.TrimSpace(cfg.Audio.WakeKeyword) == "" {
		cfg.Audio.WakeKeyword = "诺玛"
	}
	// 设置默认 TTS 提供商
	if strings.TrimSpace(cfg.TTS.Provider) == "" {
		cfg.TTS.Provider = "sherpa-onnx"
	}
	// 设置默认 Embedding 提供商
	if strings.TrimSpace(cfg.RAG.EmbedProvider) == "" {
		cfg.RAG.EmbedProvider = "ollama"
	}
	// 设置默认 Embedding 端点
	if strings.TrimSpace(cfg.RAG.EmbedEndpoint) == "" {
		cfg.RAG.EmbedEndpoint = cfg.LLM.Endpoint
	}

	// 验证所有配置的端点是否符合安全策略
	for _, endpoint := range []string{cfg.LLM.Endpoint, cfg.RAG.EmbedEndpoint} {
		if err := validateEndpointPolicy(endpoint, cfg.Security); err != nil {
			writeAPIError(w, err)
			return
		}
	}

	// 同步唤醒词相关资源文件
	if err := syncWakeKeywordArtifacts(&cfg); err != nil {
		writeAPIError(w, config.Internal("wake_keywords_sync_failed", err.Error()))
		return
	}
	// 保存更新后的配置
	if err := config.Save(h.configPath, &cfg); err != nil {
		writeAPIError(w, config.Internal("config_save_failed", err.Error()))
		return
	}
	// 发布配置更新事件到事件总线
	if h.bus != nil {
		h.bus.Publish(bus.Event{Name: "runtime.config.updated", Payload: map[string]any{"path": h.configPath}})
		publishWakeModeAnnouncement(h.bus, previous, &cfg, "settings")
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// validateEndpointPolicy 验证端点是否符合安全策略
// 检查远程端点是否在允许的主机列表中
// 参数: endpoint - 端点 URL, sec - 安全配置
// 返回: 验证错误（如果有）
func validateEndpointPolicy(endpoint string, sec config.SecurityConfig) *config.Error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" {
		return config.Invalid("invalid endpoint: " + endpoint)
	}
	host := strings.ToLower(u.Hostname())
	// 如果允许远程提供商，直接通过
	if sec.AllowRemote {
		return nil
	}
	// 检查主机是否在允许列表中
	for _, allowed := range sec.AllowedIPs {
		if strings.EqualFold(strings.TrimSpace(allowed), host) {
			return nil
		}
	}
	return config.Forbidden("remote providers are disabled by security policy")
}

// normalizeTTSToSherpa 规范化 TTS 配置为 sherpa 格式
// 自动设置默认值并转换旧格式配置
// 参数: cfg - 配置对象
// 返回: 是否发生了配置变更
func normalizeTTSToSherpa(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	changed := false
	// 将 piper 提供商转换为 sherpa-onnx
	if strings.TrimSpace(cfg.TTS.Provider) == "" || strings.EqualFold(strings.TrimSpace(cfg.TTS.Provider), "piper") {
		cfg.TTS.Provider = "sherpa-onnx"
		changed = true
	}
	// 转换旧的中文语音配置
	if strings.HasPrefix(strings.TrimSpace(cfg.TTS.Voice), "zh_CN-") {
		cfg.TTS.Voice = "zf_xiaobei"
		changed = true
	}
	// 设置默认语音
	if strings.TrimSpace(cfg.TTS.Voice) == "" {
		cfg.TTS.Voice = "zf_xiaobei"
		changed = true
	}
	// 自动查找 TTS 模型
	if strings.TrimSpace(cfg.TTS.ModelPath) == "" {
		mgr := providers.NewManager("")
		if p := mgr.FindFirst("tts", "model.onnx"); p != "" {
			cfg.TTS.ModelPath = p
			changed = true
		}
	}
	return changed
}

// syncWakeKeywordArtifacts 同步唤醒词相关资源文件
// 确保唤醒词配置文件存在并格式正确
// 参数: cfg - 配置对象
// 返回: 错误信息（如果有）
func syncWakeKeywordArtifacts(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	mgr := providers.NewManager("")
	if err := mgr.EnsureLayout(); err != nil {
		return err
	}
	// 确保唤醒词配置存在
	keywordsPath, err := bootstrap.EnsureWakeKeywordsForConfig(cfg, mgr)
	if err != nil {
		return err
	}
	if strings.TrimSpace(keywordsPath) == "" {
		modelDir := filepath.Join(mgr.Root(), "wake", "current")
		keywordsPath = filepath.Join(modelDir, "keywords.txt")
	}
	// 创建目录并写入唤醒词文件
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
	return nil
}

// autoWakeModeEnabled 检查是否启用了自动唤醒模式
// 条件: 启用唤醒词功能且模式为 wake_word_vad
// 参数: cfg - 配置对象
// 返回: 是否启用自动唤醒
func autoWakeModeEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.Audio.EnableWakeWord && strings.EqualFold(strings.TrimSpace(cfg.Audio.WakeMode), "wake_word_vad")
}

// publishWakeModeAnnouncement 发布唤醒模式变更公告
// 当唤醒模式发生变化时，通过语音播报通知用户
// 参数: b - 事件总线, previous - 旧配置, current - 新配置, source - 变更来源
func publishWakeModeAnnouncement(b *bus.Bus, previous, current *config.Config, source string) {
	if b == nil || current == nil {
		return
	}
	prev := autoWakeModeEnabled(previous)
	next := autoWakeModeEnabled(current)
	// 仅在模式变更时发布公告
	if prev == next {
		return
	}
	// 根据新模式选择提示文本
	text := "我先休息啦，有事按我头上的按钮"
	if next {
		text = "我一直都在，有事叫我哦"
	}
	b.Publish(bus.Event{Name: "assistant.say", SessionID: "system-wake-mode", TraceID: "system-wake-mode", Payload: map[string]any{"text": text, "interrupt": true, "source": source}})
}
