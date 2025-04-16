package bootstrap

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"norma/internal/audio"
	"norma/internal/config"
	"norma/internal/index"
	"norma/internal/live2d"
	"norma/internal/llm"
	"norma/internal/metrics"
	"norma/internal/providers"
	"norma/internal/runtime"
	"norma/internal/runtime/bus"
	"norma/pkg/layout"
	"xorm.io/xorm"
)

type Stack struct {
	Runtime         *runtime.Runtime
	Tracker         *metrics.Tracker
	Quality         *runtime.QualityController
	ProviderManager *providers.Manager
	Retriever       index.Retriever
	LLM             llm.Streamer
	TTS             audio.TTS
	Playback        audioPlayback
}

type WakeConfigStatus struct {
	Enabled       bool
	EffectiveMode string
	Provider      string
	Keyword       string
	Ready         bool
	Reason        string
	ModelDir      string
	KeywordsFile  string
}

type audioPlayback interface {
	Queue(sessionID, traceID string, chunk []byte) error
	Complete(sessionID, traceID string)
	Stop(sessionID, traceID, reason string)
	Wait(ctx context.Context, sessionID string) error
}

func NewStack(cfg *config.Config, engine *xorm.Engine) (*Stack, error) {
	providerMgr := providers.NewManager(cfg.Providers.Root)
	if err := providerMgr.EnsureLayout(); err != nil {
		return nil, err
	}
	log.Printf("[build] embedded whisper: %v", audio.EmbeddedWhisperEnabled())
	log.Printf("[build] embedded sherpa: %v", audio.EmbeddedSherpaEnabled())

	rt := runtime.New()
	quality := runtime.NewQualityController(
		cfg.Perf.AutoDegrade,
		cfg.Perf.LLMFirstTokenThresholdMs,
		cfg.Perf.DegradedRAGTopK,
	)
	quality.SetDegradedAudioChunkSize(cfg.Perf.DegradedAudioChunkSize)
	quality.SetDegradedEFSearch(cfg.Perf.DegradedEFSearch)
	quality.SetDegradedRetrieverFactors(cfg.Perf.DegradedRetrieverExtra, cfg.Perf.DegradedRetrieverFuse)
	quality.SetDegradedLLMModel(cfg.Perf.DegradedLLMModel)
	quality.SetDegradedTTSVoice(cfg.Perf.DegradedTTSVoice)
	tracker := metrics.NewTracker()
	runtime.AttachLatencyObserver(rt, tracker, quality)

	_ = live2d.NewBridge(rt.Bus)

	tts := chooseTTS(cfg, providerMgr, quality)
	playback := choosePlayback(rt.Bus, tts)
	streamer := chooseLLMStreamer(cfg, quality)
	retriever := chooseRetriever(cfg, engine, providerMgr)
	_ = runtime.NewOrchestratorWithPlayback(rt, retriever, streamer, tts, cfg.RAG.TopK, quality, playback)

	return &Stack{
		Runtime:         rt,
		Tracker:         tracker,
		Quality:         quality,
		ProviderManager: providerMgr,
		Retriever:       retriever,
		LLM:             streamer,
		TTS:             tts,
		Playback:        playback,
	}, nil
}

func BuildVoicePipeline(cfg *config.Config, mgr *providers.Manager) audio.Pipeline {
	return BuildVoicePipelineForMode(cfg, mgr, "")
}

func BuildVoicePipelineForMode(cfg *config.Config, mgr *providers.Manager, overrideMode string) audio.Pipeline {
	wakeStatus := ResolveWakeConfig(cfg, mgr)
	effectiveMode := wakeStatus.EffectiveMode
	if strings.TrimSpace(overrideMode) != "" {
		effectiveMode = strings.TrimSpace(overrideMode)
	}
	pipe := audio.Pipeline{
		VAD:           audio.NewEnergyVAD(0.02),
		WakeWord:      chooseWakeWord(cfg, mgr),
		ASR:           chooseASR(cfg, mgr),
		WakeMode:      effectiveMode,
		SilenceFrames: cfg.Audio.EndpointSilenceFrames,
		AllowBargeIn:  cfg.Audio.InterruptDuringSpeak,
	}
	if effectiveMode != audio.WakeModeWakeWordVAD || !wakeStatus.Enabled || !wakeStatus.Ready {
		pipe.WakeWord = nil
	}
	pipe.Preprocessors = choosePreprocessors(cfg)
	return pipe
}

func ResolveWakeConfig(cfg *config.Config, mgr *providers.Manager) WakeConfigStatus {
	status := WakeConfigStatus{
		Enabled:       cfg != nil && cfg.Audio.EnableWakeWord,
		EffectiveMode: audio.WakeModeVADOnly,
		Provider:      strings.ToLower(strings.TrimSpace(cfg.Audio.WakeProvider)),
		Keyword:       strings.TrimSpace(cfg.Audio.WakeKeyword),
	}
	if cfg == nil {
		status.Reason = "config unavailable"
		return status
	}
	if !cfg.Audio.EnableWakeWord {
		status.Reason = "audio.enable_wake_word=false"
		return status
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Audio.WakeMode))
	if mode == "" {
		mode = audio.WakeModeWakeWordVAD
	}
	if mode != audio.WakeModeWakeWordVAD {
		status.EffectiveMode = audio.WakeModeVADOnly
		status.Reason = "wake_mode is not wake_word_vad"
		return status
	}
	status.EffectiveMode = audio.WakeModeWakeWordVAD
	if status.Provider == "" {
		status.Reason = "wake provider is empty"
		return status
	}
	if status.Provider != "sherpa-onnx" {
		status.Reason = "unsupported wake provider: " + status.Provider
		return status
	}
	modelDir := strings.TrimSpace(cfg.Audio.WakeModelPath)
	if modelDir == "" && mgr != nil {
		modelDir = filepath.Join(mgr.Root(), "wake", "current")
	}
	status.ModelDir = modelDir
	encoder := ""
	decoder := ""
	joiner := ""
	tokens := ""
	keywords := strings.TrimSpace(cfg.Audio.WakeKeywordsFilePath)
	if mgr != nil {
		encoder = mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx")
		decoder = mgr.FindGlob("wake", "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx")
		joiner = mgr.FindGlob("wake", "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx")
		tokens = mgr.FindFirst("wake", "tokens.txt")
		if keywords == "" {
			keywords = mgr.FindFirst("wake", "keywords.txt", "test_keywords.txt")
		}
	}
	if encoder == "" {
		encoder = firstExistingPath(filepath.Join(modelDir, "encoder-epoch-*.onnx"), filepath.Join(modelDir, "encoder.int8.onnx"), filepath.Join(modelDir, "encoder.onnx"))
	}
	if decoder == "" {
		decoder = firstExistingPath(filepath.Join(modelDir, "decoder-epoch-*.onnx"), filepath.Join(modelDir, "decoder.int8.onnx"), filepath.Join(modelDir, "decoder.onnx"))
	}
	if joiner == "" {
		joiner = firstExistingPath(filepath.Join(modelDir, "joiner-epoch-*.onnx"), filepath.Join(modelDir, "joiner.int8.onnx"), filepath.Join(modelDir, "joiner.onnx"))
	}
	if tokens == "" {
		tokens = firstExistingPath(filepath.Join(modelDir, "tokens.txt"))
	}
	if keywords == "" {
		keywords = firstExistingPath(filepath.Join(modelDir, "keywords.txt"), filepath.Join(modelDir, "test_keywords.txt"))
	}
	status.KeywordsFile = keywords
	missing := make([]string, 0, 5)
	if encoder == "" {
		missing = append(missing, "encoder")
	}
	if decoder == "" {
		missing = append(missing, "decoder")
	}
	if joiner == "" {
		missing = append(missing, "joiner")
	}
	if tokens == "" {
		missing = append(missing, "tokens")
	}
	if keywords == "" {
		missing = append(missing, "keywords")
	}
	if len(missing) > 0 {
		status.Reason = "wake assets incomplete: " + strings.Join(missing, ",")
		return status
	}
	status.Ready = true
	status.Reason = "ready"
	return status
}

func choosePlayback(b *bus.Bus, tts audio.TTS) audioPlayback {
	provider, ok := tts.(audio.AudioFormatProvider)
	if !ok {
		return nil
	}
	sink, err := audio.NewPlatformPlaybackSink(b, provider.SampleRate(), provider.Channels(), provider.Encoding())
	if err != nil {
		log.Printf("[audio] native playback unavailable: %v", err)
		return nil
	}
	return sink
}

func chooseWakeWord(cfg *config.Config, mgr *providers.Manager) audio.WakeWord {
	status := ResolveWakeConfig(cfg, mgr)
	if !status.Enabled || status.EffectiveMode != audio.WakeModeWakeWordVAD {
		return nil
	}
	provider := status.Provider
	if provider == "" {
		return &audio.UnavailableWakeWord{Reason: "wake provider is empty"}
	}
	if provider == "sherpa-onnx" {
		log.Printf("[wake] embedded sherpa enabled=%v", audio.EmbeddedSherpaEnabled())
		modelDir := status.ModelDir
		encoder := ""
		decoder := ""
		joiner := ""
		tokens := ""
		if mgr != nil {
			encoder = mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx")
			decoder = mgr.FindGlob("wake", "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx")
			joiner = mgr.FindGlob("wake", "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx")
			tokens = mgr.FindFirst("wake", "tokens.txt")
		}
		if encoder == "" {
			encoder = firstExistingPath(filepath.Join(modelDir, "encoder-epoch-*.onnx"), filepath.Join(modelDir, "encoder.int8.onnx"), filepath.Join(modelDir, "encoder.onnx"))
		}
		if decoder == "" {
			decoder = firstExistingPath(filepath.Join(modelDir, "decoder-epoch-*.onnx"), filepath.Join(modelDir, "decoder.int8.onnx"), filepath.Join(modelDir, "decoder.onnx"))
		}
		if joiner == "" {
			joiner = firstExistingPath(filepath.Join(modelDir, "joiner-epoch-*.onnx"), filepath.Join(modelDir, "joiner.int8.onnx"), filepath.Join(modelDir, "joiner.onnx"))
		}
		if tokens == "" {
			tokens = firstExistingPath(filepath.Join(modelDir, "tokens.txt"))
		}
		keywords := status.KeywordsFile
		if encoder == "" || decoder == "" || joiner == "" || tokens == "" || keywords == "" {
			log.Printf("[wake] sherpa-onnx assets incomplete encoder=%v decoder=%v joiner=%v tokens=%v keywords=%v", encoder != "", decoder != "", joiner != "", tokens != "", keywords != "")
			return &audio.UnavailableWakeWord{Reason: "sherpa-onnx assets incomplete"}
		}
		wake := audio.NewSherpaEmbeddedWakeWord(encoder, decoder, joiner, tokens, keywords, cfg.Audio.WakeKeyword)
		if _, unavailable := wake.(*audio.UnavailableWakeWord); unavailable {
			log.Printf("[wake] embedded sherpa constructor returned unavailable keyword=%s keywords=%s", cfg.Audio.WakeKeyword, keywords)
		} else {
			log.Printf("[wake] embedded sherpa ready keyword=%s keywords=%s", cfg.Audio.WakeKeyword, keywords)
		}
		return wake
	}
	log.Printf("[wake] unsupported provider=%s", provider)
	return &audio.UnavailableWakeWord{Reason: "unsupported wake provider: " + provider}
}

func choosePreprocessors(cfg *config.Config) []audio.Preprocessor {
	if !cfg.Audio.AECEnabled && !cfg.Audio.NSEnabled {
		return nil
	}
	label := cfg.Audio.Scene
	if label == "" {
		label = "speaker"
	}
	return []audio.Preprocessor{audio.NewPassthroughAECNS(label)}
}

func chooseASR(cfg *config.Config, mgr *providers.Manager) audio.ASR {
	switch strings.ToLower(strings.TrimSpace(cfg.ASR.Provider)) {
	case "http":
		if cfg.ASR.Endpoint == "" {
			return audio.UnavailableASR{Reason: "asr.endpoint is empty"}
		}
		asr := audio.NewHTTPASR(cfg.ASR.Endpoint, time.Duration(cfg.ASR.RequestTimeoutMs)*time.Millisecond)
		asr.StreamEndpoint = strings.TrimSpace(cfg.ASR.StreamEndpoint)
		return asr
	case "whisper", "whispercpp":
		log.Printf("[asr] embedded whisper enabled=%v", audio.EmbeddedWhisperEnabled())
		model := mgr.ResolveFile("asr", "", cfg.ASR.ModelPath, "ggml-base.bin", "ggml-small.bin", "ggml-medium.bin", "ggml-large-v3.bin")
		if model == "" {
			log.Printf("[asr] whisper model not configured")
			return audio.UnavailableASR{Reason: "whisper model missing"}
		}
		if embedded, err := audio.NewWhisperEmbeddedASR(model, cfg.ASR.Language, cfg.ASR.Threads); err == nil {
			log.Printf("[asr] use embedded whisper model=%s", model)
			return embedded
		} else {
			log.Printf("[asr] embedded whisper init failed: %v", err)
		}
		binary := mgr.ResolveBinary("asr", "", cfg.ASR.BinaryPath, "whisper-cli.exe", "main.exe", "whisper-cli", "main")
		if binary == "" {
			log.Printf("[asr] whisper embedded unavailable and cli binary missing")
			return audio.UnavailableASR{Reason: "whisper embedded unavailable and cli binary missing"}
		}
		log.Printf("[asr] fallback whisper.cpp cli binary=%s model=%s", binary, model)
		return audio.NewWhisperCPPASR(binary, model, cfg.ASR.Language, cfg.ASR.Threads)
	default:
		return audio.UnavailableASR{Reason: "unsupported asr provider: " + strings.TrimSpace(cfg.ASR.Provider)}
	}
}

func chooseTTS(cfg *config.Config, mgr *providers.Manager, quality *runtime.QualityController) audio.TTS {
	fallback := &audio.UnavailableTTS{Reason: "tts unavailable"}
	switch strings.ToLower(strings.TrimSpace(cfg.TTS.Provider)) {
	case "http", "xtts":
		if cfg.TTS.Endpoint == "" {
			return fallback
		}
		return audio.NewXTTSStreamer(cfg.TTS.Endpoint, cfg.TTS.Voice, time.Duration(cfg.TTS.RequestTimeoutMs)*time.Millisecond)
	case "sherpa", "sherpa-onnx", "kokoro":
		modelPath := mgr.ResolveFile("tts", "", cfg.TTS.ModelPath, "model.onnx")
		if modelPath == "" {
			log.Printf("[tts] sherpa kokoro model not configured")
			return fallback
		}
		modelDir := filepath.Dir(modelPath)
		sid := -1
		if v := strings.TrimSpace(cfg.TTS.Voice); v != "" {
			sid = audio.ResolveSpeakerID(v)
		}
		if sid < 0 {
			sid = 45
		}
		primary := audio.NewSherpaKokoroTTS(modelDir, cfg.TTS.Voice, sid, 1.0, 2)
		log.Printf("[tts] use sherpa kokoro model_dir=%s sid=%d voice=%s", modelDir, sid, cfg.TTS.Voice)
		return primary
	default:
		return &audio.UnavailableTTS{Reason: "unsupported tts provider: " + strings.TrimSpace(cfg.TTS.Provider)}
	}
}

func chooseLLMStreamer(cfg *config.Config, quality *runtime.QualityController) llm.Streamer {
	fallback := llm.UnavailableStreamer{Reason: "llm unavailable"}
	if cfg.LLM.Provider != "ollama" {
		return fallback
	}
	if err := llm.PingOllama(cfg.LLM.Endpoint, 2*time.Second); err != nil {
		log.Printf("[llm] ollama unavailable")
		return fallback
	}

	model := strings.TrimSpace(cfg.LLM.Model)
	if model == "" {
		discovered, derr := llm.DiscoverFirstLLMModel(cfg.LLM.Endpoint, 2*time.Second)
		if derr != nil {
			log.Printf("[llm] discover model failed: %v", derr)
			return fallback
		}
		model = discovered
	}
	primary := llm.NewOllamaStreamer(cfg.LLM.Endpoint, model, time.Duration(cfg.LLM.RequestTimeoutMs)*time.Millisecond)
	degradedModel := strings.TrimSpace(cfg.Perf.DegradedLLMModel)
	if degradedModel == "" || degradedModel == model || quality == nil {
		log.Printf("[llm] use ollama provider endpoint=%s model=%s", cfg.LLM.Endpoint, model)
		return llm.FallbackStreamer{Primary: primary, Fallback: fallback}
	}

	log.Printf("[llm] use ollama provider endpoint=%s primary=%s degraded=%s", cfg.LLM.Endpoint, model, degradedModel)
	return llm.RoutedStreamer{
		Primary:     primary,
		Degraded:    llm.NewOllamaStreamer(cfg.LLM.Endpoint, degradedModel, time.Duration(cfg.LLM.RequestTimeoutMs)*time.Millisecond),
		Fallback:    fallback,
		UseDegraded: quality.IsDegraded,
	}
}

func chooseRetriever(cfg *config.Config, engine *xorm.Engine, mgr *providers.Manager) index.Retriever {
	fts := index.NewSQLiteStore(engine)

	vecPath := strings.TrimSpace(cfg.RAG.VectorIndexPath)
	if vecPath == "" {
		vecPath = layout.VectorIndexPath
	}
	if dir := filepath.Dir(vecPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("[rag] create vector index dir failed, fallback to text-only retriever: %v", err)
			return index.NewHybridRetriever(nil, fts, nil)
		}
	}

	embedder := chooseEmbedder(cfg, mgr)
	vec, err := index.NewDiskVectorStoreWithEmbedder(vecPath, embedder)
	if err != nil {
		log.Printf("[rag] init vector store failed, fallback to text-only retriever: %v", err)
		return index.NewHybridRetriever(nil, fts, nil)
	}

	log.Printf("[rag] use hybrid retriever vector=%s embed_provider=%s", vecPath, cfg.RAG.EmbedProvider)
	return index.NewHybridRetriever(vec, fts, nil)
}

func chooseEmbedder(cfg *config.Config, mgr *providers.Manager) index.Embedder {
	_ = mgr
	switch strings.ToLower(strings.TrimSpace(cfg.RAG.EmbedProvider)) {
	case "ollama":
		if strings.TrimSpace(cfg.RAG.EmbedModel) == "" {
			log.Printf("[rag] ollama embed model empty, fallback to hash embedder")
			return index.HashEmbedder{}
		}
		return index.NewOllamaEmbedder(cfg.RAG.EmbedEndpoint, cfg.RAG.EmbedModel, 15*time.Second)
	default:
		return index.HashEmbedder{}
	}
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if strings.Contains(path, "*") {
			matches, _ := filepath.Glob(path)
			for _, match := range matches {
				if _, err := os.Stat(match); err == nil {
					return match
				}
			}
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
