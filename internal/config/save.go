package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Save writes config to yaml file.
func Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	doc := map[string]any{
		"app":      map[string]any{"name": cfg.App.Name, "model_name": cfg.App.ModelName, "env": cfg.App.Env, "log_level": cfg.App.LogLevel, "log_dir": cfg.App.LogDir},
		"database": map[string]any{"path": cfg.Database.Path},
		"audio": map[string]any{
			"enable_wake_word":          cfg.Audio.EnableWakeWord,
			"mic_device_id":             cfg.Audio.MicDeviceID,
			"mic_sample_rate":           cfg.Audio.MicSampleRate,
			"mic_channel_count":         cfg.Audio.MicChannelCount,
			"wake_mode":                 cfg.Audio.WakeMode,
			"wake_provider":             cfg.Audio.WakeProvider,
			"wake_binary_path":          cfg.Audio.WakeBinaryPath,
			"wake_model_path":           cfg.Audio.WakeModelPath,
			"wake_keywords_file_path":   cfg.Audio.WakeKeywordsFilePath,
			"wake_keyword":              cfg.Audio.WakeKeyword,
			"wake_window_ms":            cfg.Audio.WakeWindowMs,
			"wake_detect_interval_ms":   cfg.Audio.WakeDetectIntervalMs,
			"endpoint_silence_frames":   cfg.Audio.EndpointSilenceFrames,
			"interrupt_during_speaking": cfg.Audio.InterruptDuringSpeak,
			"aec_enabled":               cfg.Audio.AECEnabled,
			"ns_enabled":                cfg.Audio.NSEnabled,
			"scene":                     cfg.Audio.Scene,
		},
		"asr":       map[string]any{"provider": cfg.ASR.Provider, "endpoint": cfg.ASR.Endpoint, "stream_endpoint": cfg.ASR.StreamEndpoint, "request_timeout_ms": cfg.ASR.RequestTimeoutMs, "binary_path": cfg.ASR.BinaryPath, "model_path": cfg.ASR.ModelPath, "language": cfg.ASR.Language, "threads": cfg.ASR.Threads},
		"llm":       map[string]any{"provider": cfg.LLM.Provider, "model": cfg.LLM.Model, "endpoint": cfg.LLM.Endpoint, "request_timeout_ms": cfg.LLM.RequestTimeoutMs},
		"tts":       map[string]any{"provider": cfg.TTS.Provider, "voice": cfg.TTS.Voice, "endpoint": cfg.TTS.Endpoint, "request_timeout_ms": cfg.TTS.RequestTimeoutMs, "binary_path": cfg.TTS.BinaryPath, "model_path": cfg.TTS.ModelPath, "sentence_first_enabled": cfg.TTS.SentenceFirstEnabled},
		"rag":       map[string]any{"top_k": cfg.RAG.TopK, "vector_index_path": cfg.RAG.VectorIndexPath, "embed_provider": cfg.RAG.EmbedProvider, "embed_endpoint": cfg.RAG.EmbedEndpoint, "embed_model": cfg.RAG.EmbedModel},
		"memory":    map[string]any{"stm_token_budget": cfg.Memory.STMTokenBudget, "stm_reserve_important": cfg.Memory.STMReserveImportant, "summary_auto_write": cfg.Memory.SummaryAutoWrite, "governance_compact_hours": cfg.Memory.GovernanceCompactHours},
		"perf":      map[string]any{"auto_degrade": cfg.Perf.AutoDegrade, "llm_first_token_threshold_ms": cfg.Perf.LLMFirstTokenThresholdMs, "degraded_rag_top_k": cfg.Perf.DegradedRAGTopK, "degraded_ef_search": cfg.Perf.DegradedEFSearch, "degraded_retriever_extra_factor": cfg.Perf.DegradedRetrieverExtra, "degraded_retriever_fuse_factor": cfg.Perf.DegradedRetrieverFuse, "degraded_audio_chunk_size": cfg.Perf.DegradedAudioChunkSize, "degraded_llm_model": cfg.Perf.DegradedLLMModel, "degraded_tts_voice": cfg.Perf.DegradedTTSVoice, "alert_llm_first_token_ms": cfg.Perf.AlertLLMFirstTokenMs, "alert_tts_first_chunk_ms": cfg.Perf.AlertTTSFirstChunkMs, "baseline_file": cfg.Perf.BaselineFile},
		"security":  map[string]any{"allow_remote_providers": cfg.Security.AllowRemoteProviders, "allowed_remote_hosts": cfg.Security.AllowedRemoteHosts, "tool_whitelist": cfg.Security.ToolWhitelist, "high_risk_confirm": cfg.Security.HighRiskConfirm, "purge_proof_dir": cfg.Security.PurgeProofDir},
		"providers": map[string]any{"root": cfg.Providers.Root},
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
