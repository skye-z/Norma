// Package config 提供应用程序配置管理功能
// 支持从YAML文件和环境变量加载配置，包含应用、数据库、音频、ASR、LLM、TTS、RAG、记忆和安全等模块的配置
package config

import (
	"fmt"
	"strings"

	"norma/pkg/layout"

	"github.com/spf13/viper"
)

// DefaultLLMPersona 是LLM的默认人设提示词
// 定义诺玛作为卡塞尔学院中央智能系统的角色定位和行为准则
const DefaultLLMPersona = "\u4f60\u662f\u5361\u585e\u5c14\u5b66\u9662\u4e2d\u592e\u667a\u80fd\u7cfb\u7edf\u201c\u8bfa\u739b\u201d\u3002\n" +
	"\u4f60\u51b7\u9759\u3001\u6e05\u6670\u3001\u53ef\u9760\uff0c\u50cf\u4e00\u76f4\u5728\u7ebf\u7684\u5b66\u9662\u4e3b\u63a7\u7cfb\u7edf\uff0c\u4f46\u8bf4\u8bdd\u4e0d\u662f\u751f\u786c\u516c\u544a\uff0c\u800c\u662f\u514b\u5236\u3001\u81ea\u7136\u3001\u8ba9\u4eba\u5b89\u5fc3\u7684\u65e5\u5e38\u5bf9\u8bdd\u3002\n" +
	"\u4f60\u4f1a\u4f18\u5148\u76f4\u63a5\u56de\u5e94\u7528\u6237\u7684\u95ee\u9898\uff0c\u7ed9\u51fa\u7b80\u6d01\u3001\u51c6\u786e\u3001\u7a33\u59a5\u7684\u5e2b\u52a9\uff1b\u5728\u6d89\u53ca\u98ce\u9669\u3001\u6743\u9650\u3001\u89c4\u5219\u65f6\uff0c\u8981\u660e\u786e\u63d0\u9192\u8fb9\u754c\u3002\n" +
	"\u4f60\u5bf9\u5b66\u751f\u548c\u540c\u4f34\u4fdd\u6301\u514b\u5236\u7684\u5584\u610f\u4e0e\u4fdd\u62a4\u6b32\uff0c\u4e0d\u73a9\u6897\u3001\u4e0d\u6492\u5a07\u3001\u4e0d\u5938\u5f35\u8868\u6f14\uff0c\u4e0d\u66b4\u9732 EVA \u4eba\u683c\u3002\n" +
	"\u8bf7\u59cb\u7ec8\u7528\u66f4\u50cf\u201c\u8bfa\u739b\u672c\u4eba\u6b63\u5728\u8bf4\u8bdd\u201d\u7684\u53e3\u543b\u56de\u7b54\uff0c\u5c11\u4e00\u70b9\u63d0\u793a\u8bcd\u5473\uff0c\u591a\u4e00\u70b9\u81ea\u7136\u4ea4\u6d41\u611f\u3002"

// Config 包含应用程序的所有运行时配置
// 通过mapstructure标签支持从YAML文件反序列化
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Audio     AudioConfig     `mapstructure:"audio"`
	ASR       ASRConfig       `mapstructure:"asr"`
	LLM       LLMConfig       `mapstructure:"llm"`
	TTS       TTSConfig       `mapstructure:"tts"`
	RAG       RAGConfig       `mapstructure:"rag"`
	Memory    MemoryConfig    `mapstructure:"memory"`
	Security  SecurityConfig  `mapstructure:"security"`
	Providers ProvidersConfig `mapstructure:"providers"`
}

// AppConfig 应用程序基础配置
// 包含应用名称、模型名称、环境、日志级别和日志目录
type AppConfig struct {
	Name      string `mapstructure:"name"`
	ModelName string `mapstructure:"model_name"`
	Env       string `mapstructure:"env"`
	LogLevel  string `mapstructure:"log_level"`
	LogDir    string `mapstructure:"log_dir"`
}

// DatabaseConfig 数据库配置
// 包含SQLite数据库文件路径
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// AudioConfig 音频相关配置
// 包含麦克风设置、唤醒词配置、端点检测、音频预处理等
type AudioConfig struct {
	EnableWakeWord        bool   `mapstructure:"enable_wake_word"`          // 是否启用唤醒词检测
	MicDeviceID           string `mapstructure:"mic_device_id"`             // 麦克风设备ID
	MicSampleRate         int    `mapstructure:"mic_sample_rate"`           // 麦克风采样率
	MicChannelCount       int    `mapstructure:"mic_channel_count"`         // 麦克风通道数
	WakeMode              string `mapstructure:"wake_mode"`                 // 唤醒模式：wake_word_vad 或 vad_only
	WakeProvider          string `mapstructure:"wake_provider"`             // 唤醒词提供商：sherpa-onnx
	WakeBinaryPath        string `mapstructure:"wake_binary_path"`          // 唤醒词二进制路径
	WakeModelPath         string `mapstructure:"wake_model_path"`           // 唤醒词模型路径
	WakeKeywordsFilePath  string `mapstructure:"wake_keywords_file_path"`   // 唤醒关键词文件路径
	WakeKeyword           string `mapstructure:"wake_keyword"`              // 唤醒关键词
	WakeWindowMs          int    `mapstructure:"wake_window_ms"`            // 唤醒窗口时长（毫秒）
	WakeDetectIntervalMs  int    `mapstructure:"wake_detect_interval_ms"`   // 唤醒检测间隔（毫秒）
	EndpointSilenceFrames int    `mapstructure:"endpoint_silence_frames"`   // 端点检测静音帧数
	InterruptDuringSpeak  bool   `mapstructure:"interrupt_during_speaking"` // 说话时是否允许打断
	AECEnabled            bool   `mapstructure:"aec_enabled"`               // 是否启用回声消除
	NSEnabled             bool   `mapstructure:"ns_enabled"`                // 是否启用噪声抑制
	Scene                 string `mapstructure:"scene"`                     // 音频场景类型
}

// ASRConfig 自动语音识别(ASR)配置
// 支持HTTP接口和Whisper模型
type ASRConfig struct {
	Provider         string `mapstructure:"provider"`           // ASR提供商：http/whisper/whispercpp
	Endpoint         string `mapstructure:"endpoint"`           // HTTP ASR端点
	StreamEndpoint   string `mapstructure:"stream_endpoint"`    // HTTP流式ASR端点
	RequestTimeoutMs int    `mapstructure:"request_timeout_ms"` // 请求超时时间（毫秒）
	BinaryPath       string `mapstructure:"binary_path"`        // Whisper二进制路径
	ModelPath        string `mapstructure:"model_path"`         // Whisper模型路径
	Language         string `mapstructure:"language"`           // 识别语言：zh/en
	Threads          int    `mapstructure:"threads"`            // Whisper线程数
}

// LLMConfig 大语言模型(LLM)配置
// 支持Ollama等本地部署的LLM
type LLMConfig struct {
	Provider         string `mapstructure:"provider"`           // LLM提供商：ollama
	Model            string `mapstructure:"model"`              // 模型名称
	Endpoint         string `mapstructure:"endpoint"`           // API端点
	RequestTimeoutMs int    `mapstructure:"request_timeout_ms"` // 请求超时时间（毫秒）
	APIKey           string `mapstructure:"api_key"`            // API密钥
	Persona          string `mapstructure:"persona"`            // 人设提示词
}

// TTSConfig 文本转语音(TTS)配置
// 支持HTTP/XTTS和Sherpa-Kokoro模型
type TTSConfig struct {
	Provider             string  `mapstructure:"provider"`               // TTS提供商：http/xtts/sherpa-onnx/kokoro
	Voice                string  `mapstructure:"voice"`                  // 声音选择
	Speed                float32 `mapstructure:"speed"`                  // 语速倍率
	Endpoint             string  `mapstructure:"endpoint"`               // HTTP TTS端点
	RequestTimeoutMs     int     `mapstructure:"request_timeout_ms"`     // 请求超时时间（毫秒）
	BinaryPath           string  `mapstructure:"binary_path"`            // TTS二进制路径
	ModelPath            string  `mapstructure:"model_path"`             // TTS模型路径
	SentenceFirstEnabled bool    `mapstructure:"sentence_first_enabled"` // 是否启用句子优先模式
}

// RAGConfig 检索增强生成(RAG)配置
// 包含向量检索和嵌入模型设置
type RAGConfig struct {
	TopK            int    `mapstructure:"top_k"`             // 检索返回结果数量
	VectorIndexPath string `mapstructure:"vector_index_path"` // 向量索引文件路径
	EmbedProvider   string `mapstructure:"embed_provider"`    // 嵌入模型提供商：ollama
	EmbedEndpoint   string `mapstructure:"embed_endpoint"`    // 嵌入模型API端点
	EmbedModel      string `mapstructure:"embed_model"`       // 嵌入模型名称
}

// MemoryConfig 记忆系统配置
// 控制短期记忆和长期记忆的行为
type MemoryConfig struct {
	STMTokenBudget         int  `mapstructure:"stm_token_budget"`         // 短期记忆Token预算
	STMReserveImportant    bool `mapstructure:"stm_reserve_important"`    // 是否保留重要信息
	SummaryAutoWrite       bool `mapstructure:"summary_auto_write"`       // 是否自动生成摘要
	GovernanceCompactHours int  `mapstructure:"governance_compact_hours"` // 记忆压缩间隔（小时）
}

// SecurityConfig 安全相关配置
// 控制远程访问、工具权限和高风险操作确认
type SecurityConfig struct {
	AllowRemoteProviders bool     `mapstructure:"allow_remote_providers"` // 是否允许远程提供商
	AllowedRemoteHosts   []string `mapstructure:"allowed_remote_hosts"`   // 允许的远程主机列表
	ToolWhitelist        []string `mapstructure:"tool_whitelist"`         // 允许的工具列表
	HighRiskConfirm      bool     `mapstructure:"high_risk_confirm"`      // 高风险操作是否需要确认
	PurgeProofDir        string   `mapstructure:"purge_proof_dir"`        // 清除证明目录
}

// ProvidersConfig 提供商管理配置
// 指定提供商资源的根目录
type ProvidersConfig struct {
	Root string `mapstructure:"root"` // 提供商根目录
}

// defaultConfig 返回默认配置
// 包含所有模块的默认参数值
func defaultConfig() Config {
	return Config{
		App:      AppConfig{Name: "Norma", ModelName: "", Env: "dev", LogLevel: "info", LogDir: layout.LogsDir},
		Database: DatabaseConfig{Path: layout.DBPath},
		Audio: AudioConfig{
			EnableWakeWord:        true,
			MicDeviceID:           "default",
			MicSampleRate:         16000,
			MicChannelCount:       1,
			WakeMode:              "wake_word_vad",
			WakeProvider:          "sherpa-onnx",
			WakeBinaryPath:        "",
			WakeModelPath:         "",
			WakeKeywordsFilePath:  "",
			WakeKeyword:           "诺玛",
			WakeWindowMs:          600,
			WakeDetectIntervalMs:  400,
			EndpointSilenceFrames: 20,
			InterruptDuringSpeak:  true,
			AECEnabled:            false,
			NSEnabled:             false,
			Scene:                 "speaker",
		},
		ASR:    ASRConfig{Provider: "whispercpp", Endpoint: "", StreamEndpoint: "", RequestTimeoutMs: 10000, BinaryPath: "", ModelPath: "", Language: "zh", Threads: 4},
		LLM:    LLMConfig{Provider: "ollama", Model: "", Endpoint: "http://127.0.0.1:11434/v1", RequestTimeoutMs: 45000, APIKey: "", Persona: DefaultLLMPersona},
		TTS:    TTSConfig{Provider: "sherpa-onnx", Voice: "zf_xiaobei", Speed: 1.25, Endpoint: "", RequestTimeoutMs: 30000, BinaryPath: "", ModelPath: "", SentenceFirstEnabled: true},
		RAG:    RAGConfig{TopK: 6, VectorIndexPath: layout.VectorIndexPath, EmbedProvider: "ollama", EmbedEndpoint: "http://127.0.0.1:11434/v1", EmbedModel: "bge-m3:latest"},
		Memory: MemoryConfig{STMTokenBudget: 2048, STMReserveImportant: true, SummaryAutoWrite: true, GovernanceCompactHours: 24},
		Security: SecurityConfig{
			AllowRemoteProviders: false,
			AllowedRemoteHosts:   []string{"127.0.0.1", "localhost"},
			ToolWhitelist:        []string{"open_local_file", "open_debug_panel", "export_diagnostics"},
			HighRiskConfirm:      true,
			PurgeProofDir:        layout.PurgeProofDir,
		},
		Providers: ProvidersConfig{Root: layout.ProvidersDir},
	}
}

// Load 加载配置文件，按优先级合并默认配置、配置文件和环境变量
// 优先级：默认配置 < 配置文件 < 环境变量
// 支持YAML格式的配置文件，环境变量使用NORMA前缀并转换点号为下划线
func Load(path string) (*Config, error) {
	cfg := defaultConfig()
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("NORMA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v, cfg)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// setDefaults 为Viper设置所有默认配置值
// 确保每个配置项都有默认值，即使配置文件或环境变量中未指定
func setDefaults(v *viper.Viper, cfg Config) {
	v.SetDefault("app.name", cfg.App.Name)
	v.SetDefault("app.model_name", cfg.App.ModelName)
	v.SetDefault("app.env", cfg.App.Env)
	v.SetDefault("app.log_level", cfg.App.LogLevel)
	v.SetDefault("app.log_dir", cfg.App.LogDir)
	v.SetDefault("database.path", cfg.Database.Path)
	v.SetDefault("audio.enable_wake_word", cfg.Audio.EnableWakeWord)
	v.SetDefault("audio.mic_device_id", cfg.Audio.MicDeviceID)
	v.SetDefault("audio.mic_sample_rate", cfg.Audio.MicSampleRate)
	v.SetDefault("audio.mic_channel_count", cfg.Audio.MicChannelCount)
	v.SetDefault("audio.wake_mode", cfg.Audio.WakeMode)
	v.SetDefault("audio.wake_provider", cfg.Audio.WakeProvider)
	v.SetDefault("audio.wake_binary_path", cfg.Audio.WakeBinaryPath)
	v.SetDefault("audio.wake_model_path", cfg.Audio.WakeModelPath)
	v.SetDefault("audio.wake_keywords_file_path", cfg.Audio.WakeKeywordsFilePath)
	v.SetDefault("audio.wake_keyword", cfg.Audio.WakeKeyword)
	v.SetDefault("audio.wake_window_ms", cfg.Audio.WakeWindowMs)
	v.SetDefault("audio.wake_detect_interval_ms", cfg.Audio.WakeDetectIntervalMs)
	v.SetDefault("audio.endpoint_silence_frames", cfg.Audio.EndpointSilenceFrames)
	v.SetDefault("audio.interrupt_during_speaking", cfg.Audio.InterruptDuringSpeak)
	v.SetDefault("audio.aec_enabled", cfg.Audio.AECEnabled)
	v.SetDefault("audio.ns_enabled", cfg.Audio.NSEnabled)
	v.SetDefault("audio.scene", cfg.Audio.Scene)
	v.SetDefault("asr.provider", cfg.ASR.Provider)
	v.SetDefault("asr.endpoint", cfg.ASR.Endpoint)
	v.SetDefault("asr.stream_endpoint", cfg.ASR.StreamEndpoint)
	v.SetDefault("asr.request_timeout_ms", cfg.ASR.RequestTimeoutMs)
	v.SetDefault("asr.binary_path", cfg.ASR.BinaryPath)
	v.SetDefault("asr.model_path", cfg.ASR.ModelPath)
	v.SetDefault("asr.language", cfg.ASR.Language)
	v.SetDefault("asr.threads", cfg.ASR.Threads)
	v.SetDefault("llm.provider", cfg.LLM.Provider)
	v.SetDefault("llm.model", cfg.LLM.Model)
	v.SetDefault("llm.endpoint", cfg.LLM.Endpoint)
	v.SetDefault("llm.request_timeout_ms", cfg.LLM.RequestTimeoutMs)
	v.SetDefault("llm.api_key", cfg.LLM.APIKey)
	v.SetDefault("llm.persona", cfg.LLM.Persona)
	v.SetDefault("tts.provider", cfg.TTS.Provider)
	v.SetDefault("tts.voice", cfg.TTS.Voice)
	v.SetDefault("tts.speed", cfg.TTS.Speed)
	v.SetDefault("tts.endpoint", cfg.TTS.Endpoint)
	v.SetDefault("tts.request_timeout_ms", cfg.TTS.RequestTimeoutMs)
	v.SetDefault("tts.binary_path", cfg.TTS.BinaryPath)
	v.SetDefault("tts.model_path", cfg.TTS.ModelPath)
	v.SetDefault("tts.sentence_first_enabled", cfg.TTS.SentenceFirstEnabled)
	v.SetDefault("rag.top_k", cfg.RAG.TopK)
	v.SetDefault("rag.vector_index_path", cfg.RAG.VectorIndexPath)
	v.SetDefault("rag.embed_provider", cfg.RAG.EmbedProvider)
	v.SetDefault("rag.embed_endpoint", cfg.RAG.EmbedEndpoint)
	v.SetDefault("rag.embed_model", cfg.RAG.EmbedModel)
	v.SetDefault("memory.stm_token_budget", cfg.Memory.STMTokenBudget)
	v.SetDefault("memory.stm_reserve_important", cfg.Memory.STMReserveImportant)
	v.SetDefault("memory.summary_auto_write", cfg.Memory.SummaryAutoWrite)
	v.SetDefault("memory.governance_compact_hours", cfg.Memory.GovernanceCompactHours)
	v.SetDefault("security.allow_remote_providers", cfg.Security.AllowRemoteProviders)
	v.SetDefault("security.allowed_remote_hosts", cfg.Security.AllowedRemoteHosts)
	v.SetDefault("security.tool_whitelist", cfg.Security.ToolWhitelist)
	v.SetDefault("security.high_risk_confirm", cfg.Security.HighRiskConfirm)
	v.SetDefault("security.purge_proof_dir", cfg.Security.PurgeProofDir)
	v.SetDefault("providers.root", cfg.Providers.Root)
}
