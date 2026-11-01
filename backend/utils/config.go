package config

import (
	"fmt"
	"os"
	"strings"

	"norma/pkg/layout"

	"github.com/spf13/viper"
)

// 默认人设提示词
const DefaultLLMPersona = "\u4f60\u662f\u5361\u585e\u5c14\u5b66\u9662\u4e2d\u592e\u667a\u80fd\u7cfb\u7edf\u201c\u8bfa\u739b\u201d\u3002\n" +
	"\u4f60\u51b7\u9759\u3001\u6e05\u6670\u3001\u53ef\u9760\uff0c\u50cf\u4e00\u76f4\u5728\u7ebf\u7684\u5b66\u9662\u4e3b\u63a7\u7cfb\u7edf\uff0c\u4f46\u8bf4\u8bdd\u4e0d\u662f\u751f\u65bc\u516c\u544a\uff0c\u800c\u662f\u514b\u5236\u3001\u81ea\u7136\u3001\u8ba9\u4eba\u5b89\u5fc3\u7684\u65e5\u5e38\u5bf9\u8bdd\u3002\n" +
	"\u4f60\u4f1a\u4f18\u5148\u76f4\u63a5\u56de\u5e94\u7528\u6237\u7684\u95ee\u9898\uff0c\u7ed9\u51fa\u7b80\u6d01\u3001\u51c6\u786e\u3001\u7a33\u59a5\u7684\u5e2b\u52a9\uff1b\u5728\u6d89\u53ca\u98ce\u9669\u3001\u6743\u9650\u3001\u89c4\u5219\u65f6\uff0c\u8981\u660e\u786e\u63d0\u9192\u8fb9\u754c\u3002\n" +
	"\u4f60\u5bf9\u5b66\u751f\u548c\u540c\u4f34\u4fdd\u6301\u514b\u5236\u7684\u5584\u610f\u4e0d\u4fdd\u62a4\u6b32\uff0c\u4e0d\u73a9\u6897\u3001\u4e0d\u6492\u5a07\u3001\u4e0d\u5938\u5f35\u8868\u6f14\uff0c\u4e0d\u66b4\u9732 EVA \u4eba\u683c\u3002\n" +
	"\u8bf7\u59cb\u7ec8\u7528\u66f4\u50cf\u201c\u8bfa\u739b\u672c\u4eba\u6b63\u5728\u8bf4\u8bda\u201d\u7684\u53e3\u543b\u56de\u7b54\uff0c\u5c11\u4e00\u70b9\u63d0\u793a\u8bcd\u5473\uff0c\u591a\u4e00\u70b9\u81ea\u7136\u4ea4\u6d41\u611f\u3002"

// 运行时配置
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

// 应用基础配置
type AppConfig struct {
	Name      string `mapstructure:"name"`
	ModelName string `mapstructure:"model_name"`
	Env       string `mapstructure:"env"`
	LogLevel  string `mapstructure:"log_level"`
	LogDir    string `mapstructure:"log_dir"`
}

// 数据库配置
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// 音频配置
type AudioConfig struct {
	EnableWakeWord        bool   `mapstructure:"enable_wake_word"`
	WakeMode              string `mapstructure:"wake_mode"`
	WakeKeyword           string `mapstructure:"wake_keyword"`
	EndpointSilenceFrames int    `mapstructure:"endpoint_silence_frames"`
	InterruptDuringSpeak  bool   `mapstructure:"interrupt_during_speak"`
	MicDeviceID           string `mapstructure:"mic_device_id"`
	SampleRate            int    `mapstructure:"sample_rate"`
	Channels              int    `mapstructure:"channels"`
	BufferFrames          int    `mapstructure:"buffer_frames"`
	WakeBufferFrames      int    `mapstructure:"wake_buffer_frames"`
	AEC                   string `mapstructure:"aec"`
	ANS                   string `mapstructure:"ans"`
	AGC                   string `mapstructure:"agc"`

	WakeProvider         string `mapstructure:"wake_provider"`
	WakeBinaryPath       string `mapstructure:"wake_binary_path"`
	WakeModelPath        string `mapstructure:"wake_model_path"`
	WakeKeywordsFilePath string `mapstructure:"wake_keywords_file_path"`
	WakeWindowMS         int    `mapstructure:"wake_window_ms"`
	WakeDetectIntervalMS int    `mapstructure:"wake_detect_interval_ms"`
	MicSampleRate        int    `mapstructure:"mic_sample_rate"`
	MicChannelCount      int    `mapstructure:"mic_channel_count"`
}

// 配置
type ASRConfig struct {
	Provider   string `mapstructure:"provider"`
	Language   string `mapstructure:"language"`
	BinaryPath string `mapstructure:"binary_path"`
	ModelPath  string `mapstructure:"model_path"`
	Threads    int    `mapstructure:"threads"`
	Endpoint   string `mapstructure:"endpoint"`
}

// 配置
type LLMConfig struct {
	Provider    string  `mapstructure:"provider"`
	Endpoint    string  `mapstructure:"endpoint"`
	Model       string  `mapstructure:"model"`
	APIKey      string  `mapstructure:"api_key"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

// 配置
type TTSConfig struct {
	Provider   string  `mapstructure:"provider"`
	BinaryPath string  `mapstructure:"binary_path"`
	ModelPath  string  `mapstructure:"model_path"`
	Voice      string  `mapstructure:"voice"`
	Speed      float64 `mapstructure:"speed"`
	// provider 为 genie 时使用
	Genie TTSGenieConfig `mapstructure:"genie"`
}

// Genie-TTS 配置：由应用拉起的子进程，经标准输入输出通信，不监听端口
type TTSGenieConfig struct {
	// 工作进程：打包好的可执行文件，或 python 解释器（此时配合 script）
	Worker string `mapstructure:"worker"`
	// 工作进程脚本；worker 已是打包好的可执行文件时留空
	Script       string `mapstructure:"script"`
	DataDir      string `mapstructure:"data_dir"`
	CharacterDir string `mapstructure:"character_dir"`
	Language     string `mapstructure:"language"`
	// 情绪名（对应 {{smile}} 之类的动作标记）到参考音频；default 缺省时取角色目录里 prompt_wav.json 的第一项
	References map[string]TTSGenieRef `mapstructure:"references"`
}

// 参考音频及其对应文本
type TTSGenieRef struct {
	Audio string `mapstructure:"audio"`
	Text  string `mapstructure:"text"`
}

// 配置
type RAGConfig struct {
	Provider        string `mapstructure:"provider"`
	EmbedEndpoint   string `mapstructure:"embed_endpoint"`
	EmbedModel      string `mapstructure:"embed_model"`
	EmbedProvider   string `mapstructure:"embed_provider"`
	EmbedModelDir   string `mapstructure:"embed_model_dir"`
	VectorIndexPath string `mapstructure:"vector_index_path"`
	TopK            int    `mapstructure:"top_k"`
}

// 记忆配置
type MemoryConfig struct {
	EnableSTM    bool `mapstructure:"enable_stm"`
	EnableLTM    bool `mapstructure:"enable_ltm"`
	MaxSTMTokens int  `mapstructure:"max_stm_tokens"`
	// Markdown 记忆文件路径与大小上限
	MarkdownPath     string `mapstructure:"markdown_path"`
	MarkdownMaxBytes int    `mapstructure:"markdown_max_bytes"`
}

// 安全配置
type SecurityConfig struct {
	AllowRemote bool     `mapstructure:"allow_remote"`
	AllowedIPs  []string `mapstructure:"allowed_ips"`

	ToolWhitelist   []string `mapstructure:"tool_whitelist"`
	HighRiskConfirm bool     `mapstructure:"high_risk_confirm"`
	PurgeProofDir   string   `mapstructure:"purge_proof_dir"`
}

// 配置
type ProvidersConfig struct {
	InstallMode string `mapstructure:"install_mode"`
	Root        string `mapstructure:"root"`
}

// 加载配置文件
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// 设置默认值
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config failed: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	// 应用环境变量覆盖
	applyEnvOverride(v, &cfg)

	return &cfg, nil
}

// 设置默认值
func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "Norma")
	v.SetDefault("app.model_name", "default")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.log_dir", layout.LogsDir)

	v.SetDefault("database.path", layout.DBPath)

	v.SetDefault("audio.enable_wake_word", true)
	v.SetDefault("audio.wake_mode", "vad")
	v.SetDefault("audio.wake_keyword", "hey norma")
	v.SetDefault("audio.endpoint_silence_frames", 40)
	v.SetDefault("audio.interrupt_during_speak", true)
	v.SetDefault("audio.sample_rate", 16000)
	v.SetDefault("audio.channels", 1)
	v.SetDefault("audio.buffer_frames", 512)
	v.SetDefault("audio.wake_buffer_frames", 10)

	v.SetDefault("asr.provider", "whisper")
	v.SetDefault("asr.language", "zh")
	v.SetDefault("asr.threads", 4)

	v.SetDefault("llm.provider", "ollama")
	v.SetDefault("llm.endpoint", "http://127.0.0.1:11434")
	v.SetDefault("llm.max_tokens", 4096)
	v.SetDefault("llm.temperature", 0.7)

	v.SetDefault("tts.provider", "genie")
	v.SetDefault("tts.genie.worker", layout.GenieWorkerPath)
	v.SetDefault("tts.genie.data_dir", layout.GenieDataDir)
	v.SetDefault("tts.genie.character_dir", layout.GenieCharacterDir)
	v.SetDefault("tts.genie.language", "zh")
	v.SetDefault("tts.speed", 1.25)

	v.SetDefault("rag.provider", "hybrid")
	v.SetDefault("rag.embed_provider", "static")
	v.SetDefault("rag.embed_model_dir", layout.StaticEmbedModelDir)
	v.SetDefault("rag.vector_index_path", layout.VectorIndexPath)

	v.SetDefault("memory.markdown_path", layout.MemoryPath)
	v.SetDefault("memory.markdown_max_bytes", 16*1024)
	v.SetDefault("memory.enable_stm", true)
	v.SetDefault("memory.enable_ltm", true)
	v.SetDefault("memory.max_stm_tokens", 4096)

	v.SetDefault("security.allow_remote", false)
}

// 应用环境变量覆盖
func applyEnvOverride(v *viper.Viper, cfg *Config) {
	// 环境变量覆盖逻辑
	if env := strings.TrimSpace(os.Getenv("NORMA_LLM_ENDPOINT")); env != "" {
		cfg.LLM.Endpoint = env
	}
	if env := strings.TrimSpace(os.Getenv("NORMA_LLM_API_KEY")); env != "" {
		cfg.LLM.APIKey = env
	}
	if env := strings.TrimSpace(os.Getenv("NORMA_LLM_MODEL")); env != "" {
		cfg.LLM.Model = env
	}
	if env := strings.TrimSpace(os.Getenv("NORMA_LOG_LEVEL")); env != "" {
		cfg.App.LogLevel = env
	}
}
