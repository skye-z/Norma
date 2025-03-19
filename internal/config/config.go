package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"norma/pkg/layout"
)

// Config contains all runtime configuration.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Audio     AudioConfig     `mapstructure:"audio"`
	ASR       ASRConfig       `mapstructure:"asr"`
	LLM       LLMConfig       `mapstructure:"llm"`
	TTS       TTSConfig       `mapstructure:"tts"`
	RAG       RAGConfig       `mapstructure:"rag"`
	Memory    MemoryConfig    `mapstructure:"memory"`
	Perf      PerfConfig      `mapstructure:"perf"`
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
}

type TTSConfig struct {
	Provider             string `mapstructure:"provider"`
	Voice                string `mapstructure:"voice"`
	Endpoint             string `mapstructure:"endpoint"`
	RequestTimeoutMs     int    `mapstructure:"request_timeout_ms"`
	BinaryPath           string `mapstructure:"binary_path"`
	ModelPath            string `mapstructure:"model_path"`
	SentenceFirstEnabled bool   `mapstructure:"sentence_first_enabled"`
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

type PerfConfig struct {
	AutoDegrade              bool   `mapstructure:"auto_degrade"`
	LLMFirstTokenThresholdMs int    `mapstructure:"llm_first_token_threshold_ms"`
	DegradedRAGTopK          int    `mapstructure:"degraded_rag_top_k"`
	DegradedEFSearch         int    `mapstructure:"degraded_ef_search"`
	DegradedRetrieverExtra   int    `mapstructure:"degraded_retriever_extra_factor"`
	DegradedRetrieverFuse    int    `mapstructure:"degraded_retriever_fuse_factor"`
	DegradedAudioChunkSize   int    `mapstructure:"degraded_audio_chunk_size"`
	DegradedLLMModel         string `mapstructure:"degraded_llm_model"`
	DegradedTTSVoice         string `mapstructure:"degraded_tts_voice"`
	AlertLLMFirstTokenMs     int    `mapstructure:"alert_llm_first_token_ms"`
	AlertTTSFirstChunkMs     int    `mapstructure:"alert_tts_first_chunk_ms"`
	BaselineFile             string `mapstructure:"baseline_file"`
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
			WakeWindowMs:          2000,
			WakeDetectIntervalMs:  400,
			EndpointSilenceFrames: 4,
			InterruptDuringSpeak:  true,
			AECEnabled:            false,
			NSEnabled:             false,
			Scene:                 "speaker",
		},
		ASR:    ASRConfig{Provider: "whispercpp", Endpoint: "", StreamEndpoint: "", RequestTimeoutMs: 10000, BinaryPath: "", ModelPath: "", Language: "zh", Threads: 4},
		LLM:    LLMConfig{Provider: "ollama", Model: "", Endpoint: "http://127.0.0.1:11434", RequestTimeoutMs: 45000},
		TTS:    TTSConfig{Provider: "piper", Voice: "zh_CN-huayan-medium", Endpoint: "", RequestTimeoutMs: 30000, BinaryPath: "", ModelPath: "", SentenceFirstEnabled: true},
		RAG:    RAGConfig{TopK: 6, VectorIndexPath: layout.VectorIndexPath, EmbedProvider: "ollama", EmbedEndpoint: "http://127.0.0.1:11434", EmbedModel: "bge-m3:latest"},
		Memory: MemoryConfig{STMTokenBudget: 2048, STMReserveImportant: true, SummaryAutoWrite: true, GovernanceCompactHours: 24},
		Perf: PerfConfig{
			AutoDegrade:              true,
			LLMFirstTokenThresholdMs: 1200,
			DegradedRAGTopK:          4,
			DegradedEFSearch:         24,
			DegradedRetrieverExtra:   1,
			DegradedRetrieverFuse:    2,
			DegradedAudioChunkSize:   4096,
			DegradedLLMModel:         "",
			DegradedTTSVoice:         "zh_CN-huayan-x_low",
			AlertLLMFirstTokenMs:     1500,
			AlertTTSFirstChunkMs:     1200,
			BaselineFile:             "build/perf-baseline.json",
		},
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
	v.SetDefault("tts.provider", cfg.TTS.Provider)
	v.SetDefault("tts.voice", cfg.TTS.Voice)
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
	v.SetDefault("perf.auto_degrade", cfg.Perf.AutoDegrade)
	v.SetDefault("perf.llm_first_token_threshold_ms", cfg.Perf.LLMFirstTokenThresholdMs)
	v.SetDefault("perf.degraded_rag_top_k", cfg.Perf.DegradedRAGTopK)
	v.SetDefault("perf.degraded_ef_search", cfg.Perf.DegradedEFSearch)
	v.SetDefault("perf.degraded_retriever_extra_factor", cfg.Perf.DegradedRetrieverExtra)
	v.SetDefault("perf.degraded_retriever_fuse_factor", cfg.Perf.DegradedRetrieverFuse)
	v.SetDefault("perf.degraded_audio_chunk_size", cfg.Perf.DegradedAudioChunkSize)
	v.SetDefault("perf.degraded_llm_model", cfg.Perf.DegradedLLMModel)
	v.SetDefault("perf.degraded_tts_voice", cfg.Perf.DegradedTTSVoice)
	v.SetDefault("perf.alert_llm_first_token_ms", cfg.Perf.AlertLLMFirstTokenMs)
	v.SetDefault("perf.alert_tts_first_chunk_ms", cfg.Perf.AlertTTSFirstChunkMs)
	v.SetDefault("perf.baseline_file", cfg.Perf.BaselineFile)
	v.SetDefault("security.allow_remote_providers", cfg.Security.AllowRemoteProviders)
	v.SetDefault("security.allowed_remote_hosts", cfg.Security.AllowedRemoteHosts)
	v.SetDefault("security.tool_whitelist", cfg.Security.ToolWhitelist)
	v.SetDefault("security.high_risk_confirm", cfg.Security.HighRiskConfirm)
	v.SetDefault("security.purge_proof_dir", cfg.Security.PurgeProofDir)
	v.SetDefault("providers.root", cfg.Providers.Root)
}
