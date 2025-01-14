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
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type AudioConfig struct {
	EnableWakeWord bool `mapstructure:"enable_wake_word"`
}

type ASRConfig struct {
	Provider         string `mapstructure:"provider"`
	Endpoint         string `mapstructure:"endpoint"`
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
	Provider         string `mapstructure:"provider"`
	Voice            string `mapstructure:"voice"`
	Endpoint         string `mapstructure:"endpoint"`
	RequestTimeoutMs int    `mapstructure:"request_timeout_ms"`
}

type RAGConfig struct {
	TopK int `mapstructure:"top_k"`
}

type MemoryConfig struct {
	STMTokenBudget int `mapstructure:"stm_token_budget"`
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
		},
		Database: DatabaseConfig{Path: "data/norma.db"},
		Audio:    AudioConfig{EnableWakeWord: true},
		ASR: ASRConfig{
			Provider:         "mock",
			Endpoint:         "http://127.0.0.1:8081",
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
			Provider:         "mock",
			Voice:            "norma",
			Endpoint:         "http://127.0.0.1:8020",
			RequestTimeoutMs: 30000,
		},
		RAG:    RAGConfig{TopK: 6},
		Memory: MemoryConfig{STMTokenBudget: 2048},
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
		// Allow missing config file; defaults + env are still valid.
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

	v.SetDefault("database.path", cfg.Database.Path)

	v.SetDefault("audio.enable_wake_word", cfg.Audio.EnableWakeWord)

	v.SetDefault("asr.provider", cfg.ASR.Provider)
	v.SetDefault("asr.endpoint", cfg.ASR.Endpoint)
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

	v.SetDefault("rag.top_k", cfg.RAG.TopK)

	v.SetDefault("memory.stm_token_budget", cfg.Memory.STMTokenBudget)

	v.SetDefault("perf.auto_degrade", cfg.Perf.AutoDegrade)
	v.SetDefault("perf.llm_first_token_threshold_ms", cfg.Perf.LLMFirstTokenThresholdMs)
	v.SetDefault("perf.degraded_rag_top_k", cfg.Perf.DegradedRAGTopK)
}
