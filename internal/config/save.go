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
		"app": map[string]any{
			"name":      cfg.App.Name,
			"env":       cfg.App.Env,
			"log_level": cfg.App.LogLevel,
			"log_dir":   cfg.App.LogDir,
		},
		"database": map[string]any{
			"path": cfg.Database.Path,
		},
		"audio": map[string]any{
			"enable_wake_word":          cfg.Audio.EnableWakeWord,
			"wake_mode":                 cfg.Audio.WakeMode,
			"endpoint_silence_frames":   cfg.Audio.EndpointSilenceFrames,
			"interrupt_during_speaking": cfg.Audio.InterruptDuringSpeak,
			"aec_enabled":               cfg.Audio.AECEnabled,
			"ns_enabled":                cfg.Audio.NSEnabled,
			"scene":                     cfg.Audio.Scene,
		},
		"asr": map[string]any{
			"provider":           cfg.ASR.Provider,
			"endpoint":           cfg.ASR.Endpoint,
			"stream_endpoint":    cfg.ASR.StreamEndpoint,
			"request_timeout_ms": cfg.ASR.RequestTimeoutMs,
			"binary_path":        cfg.ASR.BinaryPath,
			"model_path":         cfg.ASR.ModelPath,
			"language":           cfg.ASR.Language,
			"threads":            cfg.ASR.Threads,
		},
		"llm": map[string]any{
			"provider":           cfg.LLM.Provider,
			"model":              cfg.LLM.Model,
			"endpoint":           cfg.LLM.Endpoint,
			"request_timeout_ms": cfg.LLM.RequestTimeoutMs,
		},
		"tts": map[string]any{
			"provider":               cfg.TTS.Provider,
			"voice":                  cfg.TTS.Voice,
			"endpoint":               cfg.TTS.Endpoint,
			"request_timeout_ms":     cfg.TTS.RequestTimeoutMs,
			"sentence_first_enabled": cfg.TTS.SentenceFirstEnabled,
		},
		"rag": map[string]any{
			"top_k":             cfg.RAG.TopK,
			"vector_index_path": cfg.RAG.VectorIndexPath,
		},
		"memory": map[string]any{
			"stm_token_budget":         cfg.Memory.STMTokenBudget,
			"stm_reserve_important":    cfg.Memory.STMReserveImportant,
			"summary_auto_write":       cfg.Memory.SummaryAutoWrite,
			"governance_compact_hours": cfg.Memory.GovernanceCompactHours,
		},
		"perf": map[string]any{
			"auto_degrade":                 cfg.Perf.AutoDegrade,
			"llm_first_token_threshold_ms": cfg.Perf.LLMFirstTokenThresholdMs,
			"degraded_rag_top_k":           cfg.Perf.DegradedRAGTopK,
		},
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
