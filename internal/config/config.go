package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config contains all runtime configuration.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Audio    AudioConfig    `mapstructure:"audio"`
	ASR      ASRConfig      `mapstructure:"asr"`
	LLM      LLMConfig      `mapstructure:"llm"`
	TTS      TTSConfig      `mapstructure:"tts"`
	RAG      RAGConfig      `mapstructure:"rag"`
	Memory   MemoryConfig   `mapstructure:"memory"`
	Perf     PerfConfig     `mapstructure:"perf"`
}

type AppConfig struct {
	Name     string `mapstructure:"name"`
	Env      string `mapstructure:"env"`
	LogLevel string `mapstructure:"log_level"`
	LogDir   string `mapstructure:"log_dir"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type AudioConfig struct {
	EnableWakeWord        bool   `mapstructure:"enable_wake_word"`
	WakeMode              string `mapstructure:"wake_mode"`
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
}

type TTSConfig struct {
	Provider             string `mapstructure:"provider"`
	Voice                string `mapstructure:"voice"`
	Endpoint             string `mapstructure:"endpoint"`
	RequestTimeoutMs     int    `mapstructure:"request_timeout_ms"`
	SentenceFirstEnabled bool   `mapstructure:"sentence_first_enabled"`
}

type RAGConfig struct {
	TopK            int    `mapstructure:"top_k"`
	VectorIndexPath string `mapstructure:"vector_index_path"`
}

type MemoryConfig struct {
	STMTokenBudget         int  `mapstructure:"stm_token_budget"`
	STMReserveImportant    bool `mapstructure:"stm_reserve_important"`
	SummaryAutoWrite       bool `mapstructure:"summary_auto_write"`
	GovernanceCompactHours int  `mapstructure:"governance_compact_hours"`
}

type PerfConfig struct {
	AutoDegrade              bool `mapstructure:"auto_degrade"`
	LLMFirstTokenThresholdMs int  `mapstructure:"llm_first_token_threshold_ms"`
	DegradedRAGTopK          int  `mapstructure:"degraded_rag_top_k"`
}

func defaultConfig() Config {
	return Config{
		App: AppConfig{
			Name:     "Norma",
			Env:      "dev",
			LogLevel: "info",
			LogDir:   "logs",
		},
		Database: DatabaseConfig{Path: "data/norma.db"},
		Audio: AudioConfig{
			EnableWakeWord:        true,
			WakeMode:              "wake_word_vad",
			EndpointSilenceFrames: 4,
			InterruptDuringSpeak:  true,
			AECEnabled:            false,
			NSEnabled:             false,
			Scene:                 "speaker",
		},
		ASR: ASRConfig{
			Provider:         "mock",
			Endpoint:         "http://127.0.0.1:8081",
			StreamEndpoint:   "",
			RequestTimeoutMs: 10000,
			BinaryPath:       "",
			ModelPath:        "",
			Language:         "zh",
			Threads:          4,
		},
		LLM: LLMConfig{
			Provider:         "ollama",
			Model:            "",
			Endpoint:         "http://127.0.0.1:11434",
			RequestTimeoutMs: 45000,
		},
		TTS: TTSConfig{
			Provider:             "mock",
			Voice:                "norma",
			Endpoint:             "http://127.0.0.1:8020",
			RequestTimeoutMs:     30000,
			SentenceFirstEnabled: true,
		},
		RAG: RAGConfig{
			TopK:            6,
			VectorIndexPath: "data/vector_index.json",
		},
		Memory: MemoryConfig{
			STMTokenBudget:         2048,
			STMReserveImportant:    true,
			SummaryAutoWrite:       true,
			GovernanceCompactHours: 24,
		},
		Perf: PerfConfig{
			AutoDegrade:              true,
			LLMFirstTokenThresholdMs: 1200,
			DegradedRAGTopK:          4,
		},
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
	v.SetDefault("app.env", cfg.App.Env)
	v.SetDefault("app.log_level", cfg.App.LogLevel)
	v.SetDefault("app.log_dir", cfg.App.LogDir)

	v.SetDefault("database.path", cfg.Database.Path)

	v.SetDefault("audio.enable_wake_word", cfg.Audio.EnableWakeWord)
	v.SetDefault("audio.wake_mode", cfg.Audio.WakeMode)
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

	v.SetDefault("tts.provider", cfg.TTS.Provider)
	v.SetDefault("tts.voice", cfg.TTS.Voice)
	v.SetDefault("tts.endpoint", cfg.TTS.Endpoint)
	v.SetDefault("tts.request_timeout_ms", cfg.TTS.RequestTimeoutMs)
	v.SetDefault("tts.sentence_first_enabled", cfg.TTS.SentenceFirstEnabled)

	v.SetDefault("rag.top_k", cfg.RAG.TopK)
	v.SetDefault("rag.vector_index_path", cfg.RAG.VectorIndexPath)

	v.SetDefault("memory.stm_token_budget", cfg.Memory.STMTokenBudget)
	v.SetDefault("memory.stm_reserve_important", cfg.Memory.STMReserveImportant)
	v.SetDefault("memory.summary_auto_write", cfg.Memory.SummaryAutoWrite)
	v.SetDefault("memory.governance_compact_hours", cfg.Memory.GovernanceCompactHours)

	v.SetDefault("perf.auto_degrade", cfg.Perf.AutoDegrade)
	v.SetDefault("perf.llm_first_token_threshold_ms", cfg.Perf.LLMFirstTokenThresholdMs)
	v.SetDefault("perf.degraded_rag_top_k", cfg.Perf.DegradedRAGTopK)
}
