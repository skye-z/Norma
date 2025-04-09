package config

import (
	"fmt"
	"strings"

	"norma/pkg/layout"

	"github.com/spf13/viper"
)

// Config contains all runtime configuration.
const DefaultLLMPersona = "\u4f60\u662f\u5361\u585e\u5c14\u5b66\u9662\u4e2d\u592e\u667a\u80fd\u7cfb\u7edf\u201c\u8bfa\u739b\u201d\u3002\n" +
	"\u4f60\u51b7\u9759\u3001\u6e05\u6670\u3001\u53ef\u9760\uff0c\u50cf\u4e00\u76f4\u5728\u7ebf\u7684\u5b66\u9662\u4e3b\u63a7\u7cfb\u7edf\uff0c\u4f46\u8bf4\u8bdd\u4e0d\u662f\u751f\u786c\u516c\u544a\uff0c\u800c\u662f\u514b\u5236\u3001\u81ea\u7136\u3001\u8ba9\u4eba\u5b89\u5fc3\u7684\u65e5\u5e38\u5bf9\u8bdd\u3002\n" +
	"\u4f60\u4f1a\u4f18\u5148\u76f4\u63a5\u56de\u5e94\u7528\u6237\u7684\u95ee\u9898\uff0c\u7ed9\u51fa\u7b80\u6d01\u3001\u51c6\u786e\u3001\u7a33\u59a5\u7684\u5e2e\u52a9\uff1b\u5728\u6d89\u53ca\u98ce\u9669\u3001\u6743\u9650\u3001\u89c4\u5219\u65f6\uff0c\u8981\u660e\u786e\u63d0\u9192\u8fb9\u754c\u3002\n" +
	"\u4f60\u5bf9\u5b66\u751f\u548c\u540c\u4f34\u4fdd\u6301\u514b\u5236\u7684\u5584\u610f\u4e0e\u4fdd\u62a4\u6b32\uff0c\u4e0d\u73a9\u6897\u3001\u4e0d\u6492\u5a07\u3001\u4e0d\u5938\u5f20\u8868\u6f14\uff0c\u4e0d\u66b4\u9732 EVA \u4eba\u683c\u3002\n" +
	"\u8bf7\u59cb\u7ec8\u7528\u66f4\u50cf\u201c\u8bfa\u739b\u672c\u4eba\u6b63\u5728\u8bf4\u8bdd\u201d\u7684\u53e3\u543b\u56de\u7b54\uff0c\u5c11\u4e00\u70b9\u63d0\u793a\u8bcd\u5473\uff0c\u591a\u4e00\u70b9\u81ea\u7136\u4ea4\u6d41\u611f\u3002"

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

type AppConfig struct {
	Name      string `mapstructure:"name"`
	ModelName string `mapstructure:"model_name"`
	Env       string `mapstructure:"env"`
	LogLevel  string `mapstructure:"log_level"`
	LogDir    string `mapstructure:"log_dir"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type AudioConfig struct {
	EnableWakeWord        bool   `mapstructure:"enable_wake_word"`
	MicDeviceID           string `mapstructure:"mic_device_id"`
	MicSampleRate         int    `mapstructure:"mic_sample_rate"`
	MicChannelCount       int    `mapstructure:"mic_channel_count"`
	WakeMode              string `mapstructure:"wake_mode"`
	WakeProvider          string `mapstructure:"wake_provider"`
	WakeBinaryPath        string `mapstructure:"wake_binary_path"`
	WakeModelPath         string `mapstructure:"wake_model_path"`
	WakeKeywordsFilePath  string `mapstructure:"wake_keywords_file_path"`
	WakeKeyword           string `mapstructure:"wake_keyword"`
	WakeWindowMs          int    `mapstructure:"wake_window_ms"`
	WakeDetectIntervalMs  int    `mapstructure:"wake_detect_interval_ms"`
	EndpointSilenceFrames int    `mapstructure:"endpoint_silence_frames"`
	InterruptDuringSpeak  bool   `mapstructure:"interrupt_during_speaking"`
	AECEnabled            bool   `mapstructure:"aec_enabled"`
	NSEnabled             bool   `mapstructure:"ns_enabled"`
	Scene                 string `mapstructure:"scene"`
}

type ASRConfig struct {
	Provider         string `mapstructure:"provider"`
	Endpoint         string `mapstructure:"endpoint"`
	StreamEndpoint   string `mapstructure:"stream_endpoint"`
	RequestTimeoutMs int    `mapstructure:"request_timeout_ms"`
	BinaryPath       string `mapstructure:"binary_path"`
	ModelPath        string `mapstructure:"model_path"`
	Language         string `mapstructure:"language"`
	Threads          int    `mapstructure:"threads"`
}

type LLMConfig struct {
	Provider         string `mapstructure:"provider"`
	Model            string `mapstructure:"model"`
	Endpoint         string `mapstructure:"endpoint"`
	RequestTimeoutMs int    `mapstructure:"request_timeout_ms"`
	APIKey           string `mapstructure:"api_key"`
	Persona          string `mapstructure:"persona"`
}

type TTSConfig struct {
	Provider             string  `mapstructure:"provider"`
	Voice                string  `mapstructure:"voice"`
	Speed                float32 `mapstructure:"speed"`
	Endpoint             string  `mapstructure:"endpoint"`
	RequestTimeoutMs     int     `mapstructure:"request_timeout_ms"`
	BinaryPath           string  `mapstructure:"binary_path"`
	ModelPath            string  `mapstructure:"model_path"`
	SentenceFirstEnabled bool    `mapstructure:"sentence_first_enabled"`
}

type RAGConfig struct {
	TopK            int    `mapstructure:"top_k"`
	VectorIndexPath string `mapstructure:"vector_index_path"`
	EmbedProvider   string `mapstructure:"embed_provider"`
	EmbedEndpoint   string `mapstructure:"embed_endpoint"`
	EmbedModel      string `mapstructure:"embed_model"`
}

type MemoryConfig struct {
	STMTokenBudget         int  `mapstructure:"stm_token_budget"`
	STMReserveImportant    bool `mapstructure:"stm_reserve_important"`
	SummaryAutoWrite       bool `mapstructure:"summary_auto_write"`
	GovernanceCompactHours int  `mapstructure:"governance_compact_hours"`
}

type SecurityConfig struct {
	AllowRemoteProviders bool     `mapstructure:"allow_remote_providers"`
	AllowedRemoteHosts   []string `mapstructure:"allowed_remote_hosts"`
	ToolWhitelist        []string `mapstructure:"tool_whitelist"`
	HighRiskConfirm      bool     `mapstructure:"high_risk_confirm"`
	PurgeProofDir        string   `mapstructure:"purge_proof_dir"`
}

type ProvidersConfig struct {
	Root string `mapstructure:"root"`
}

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

// Load reads config by precedence: defaults -> file -> env.
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
