package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"norma/backend/core/audio"
	"norma/backend/utils"
	"norma/backend/core/providers"
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

func TestResolveWakeConfigCreatesMissingKeywordsFile(t *testing.T) {
	root := t.TempDir()
	wakeDir := filepath.Join(root, "wake")
	if err := os.MkdirAll(wakeDir, 0o755); err != nil {
		t.Fatalf("mkdir wake dir: %v", err)
	}
	for _, name := range []string{"encoder.onnx", "decoder.onnx", "joiner.onnx", "tokens.txt"} {
		if err := os.WriteFile(filepath.Join(wakeDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	cfg := &config.Config{}
	cfg.Audio.EnableWakeWord = true
	cfg.Audio.WakeMode = audio.WakeModeWakeWordVAD
	cfg.Audio.WakeProvider = "sherpa-onnx"
	cfg.Audio.WakeModelPath = wakeDir
	cfg.Audio.WakeKeyword = "诺玛"

	status := ResolveWakeConfig(cfg, providers.NewManager(root))
	if !status.Ready {
		t.Fatalf("expected wake assets to be ready, got reason=%s", status.Reason)
	}
	data, err := os.ReadFile(filepath.Join(wakeDir, "keywords.txt"))
	if err != nil {
		t.Fatalf("read keywords: %v", err)
	}
	if string(data) != "n uò m ǎ @诺玛\n" {
		t.Fatalf("unexpected keywords content: %q", string(data))
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
