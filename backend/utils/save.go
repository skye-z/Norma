// Package config 配置管理
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Save 保存配置到 YAML 文件
func Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	doc := map[string]any{
		"app":      map[string]any{"name": cfg.App.Name, "model_name": cfg.App.ModelName, "env": cfg.App.Env, "log_level": cfg.App.LogLevel, "log_dir": cfg.App.LogDir},
		"database": map[string]any{"path": cfg.Database.Path},
		"audio": map[string]any{
			"enable_wake_word":        cfg.Audio.EnableWakeWord,
			"wake_mode":               cfg.Audio.WakeMode,
			"wake_keyword":            cfg.Audio.WakeKeyword,
			"endpoint_silence_frames": cfg.Audio.EndpointSilenceFrames,
			"interrupt_during_speak":  cfg.Audio.InterruptDuringSpeak,
			"mic_device_id":           cfg.Audio.MicDeviceID,
			"sample_rate":             cfg.Audio.SampleRate,
			"channels":                cfg.Audio.Channels,
			"buffer_frames":           cfg.Audio.BufferFrames,
			"wake_buffer_frames":      cfg.Audio.WakeBufferFrames,
			"aec":                     cfg.Audio.AEC,
			"ans":                     cfg.Audio.ANS,
			"agc":                     cfg.Audio.AGC,
		},
		"asr":       map[string]any{"provider": cfg.ASR.Provider, "language": cfg.ASR.Language, "binary_path": cfg.ASR.BinaryPath, "model_path": cfg.ASR.ModelPath, "threads": cfg.ASR.Threads},
		"llm":       map[string]any{"provider": cfg.LLM.Provider, "model": cfg.LLM.Model, "endpoint": cfg.LLM.Endpoint, "max_tokens": cfg.LLM.MaxTokens, "temperature": cfg.LLM.Temperature},
		"tts":       map[string]any{"provider": cfg.TTS.Provider, "voice": cfg.TTS.Voice, "speed": cfg.TTS.Speed, "binary_path": cfg.TTS.BinaryPath, "model_path": cfg.TTS.ModelPath},
		"rag":       map[string]any{"provider": cfg.RAG.Provider, "vector_index_path": cfg.RAG.VectorIndexPath, "embed_provider": cfg.RAG.EmbedProvider, "embed_endpoint": cfg.RAG.EmbedEndpoint, "embed_model": cfg.RAG.EmbedModel},
		"memory":    map[string]any{"enable_stm": cfg.Memory.EnableSTM, "enable_ltm": cfg.Memory.EnableLTM, "max_stm_tokens": cfg.Memory.MaxSTMTokens},
		"security":  map[string]any{"allow_remote": cfg.Security.AllowRemote, "allowed_ips": cfg.Security.AllowedIPs},
		"providers": map[string]any{"install_mode": cfg.Providers.InstallMode},
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
