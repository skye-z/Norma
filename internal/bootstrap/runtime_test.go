package bootstrap

import (
	"path/filepath"
	"testing"

	"norma/internal/audio"
	"norma/internal/config"
	"norma/internal/providers"
)

func TestResolveWakeConfigDisabledFallsBackToVAD(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.EnableWakeWord = false
	cfg.Audio.WakeMode = audio.WakeModeWakeWordVAD
	status := ResolveWakeConfig(cfg, providers.NewManager(t.TempDir()))
	if status.Enabled {
		t.Fatalf("expected wake disabled")
	}
	if status.EffectiveMode != audio.WakeModeVADOnly {
		t.Fatalf("expected vad mode, got %s", status.EffectiveMode)
	}
}

func TestResolveWakeConfigRequiresAssets(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Audio.EnableWakeWord = true
	cfg.Audio.WakeMode = audio.WakeModeWakeWordVAD
	cfg.Audio.WakeProvider = "sherpa-onnx"
	cfg.Audio.WakeModelPath = filepath.Join(root, "wake")
	status := ResolveWakeConfig(cfg, providers.NewManager(root))
	if status.Ready {
		t.Fatalf("expected wake assets to be missing")
	}
	if status.EffectiveMode != audio.WakeModeWakeWordVAD {
		t.Fatalf("expected wake mode to remain wake_word_vad, got %s", status.EffectiveMode)
	}
	if status.Reason == "" {
		t.Fatalf("expected missing asset reason")
	}
}

func TestBuildVoicePipelineDisablesWakeWhenConfigNotEffective(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.EnableWakeWord = false
	cfg.Audio.WakeMode = audio.WakeModeWakeWordVAD
	cfg.ASR.Provider = "http"
	cfg.ASR.Endpoint = "http://127.0.0.1:9999"
	pipe := BuildVoicePipeline(cfg, providers.NewManager(t.TempDir()))
	if pipe.WakeWord != nil {
		t.Fatalf("expected wake word to be nil when disabled")
	}
	if pipe.WakeMode != audio.WakeModeVADOnly {
		t.Fatalf("expected vad only mode, got %s", pipe.WakeMode)
	}
}
