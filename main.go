package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xorm.io/xorm"

	"norma/internal/audio"
	"norma/internal/config"
	"norma/internal/db"
	"norma/internal/desktop"
	"norma/internal/index"
	"norma/internal/live2d"
	"norma/internal/llm"
	"norma/internal/metrics"
	"norma/internal/ops"
	"norma/internal/providers"
	"norma/internal/runtime"
	"norma/internal/runtime/bus"
	"norma/pkg/layout"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	cfgPath := flag.String("config", layout.ConfigFileName, "path to config file")
	demo := flag.Bool("demo", false, "run event-chain demo and exit")
	flag.Parse()

	if !*demo {
		if err := desktop.Run(frontendAssets); err != nil {
			log.Fatalf("start desktop app: %v", err)
		}
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	providerMgr := providers.NewManager(cfg.Providers.Root)
	if err := providerMgr.EnsureLayout(); err != nil {
		log.Printf("[providers] ensure layout failed: %v", err)
	}

	logCloser, err := ops.SetupLogRouting(cfg.App.LogDir)
	if err != nil {
		log.Fatalf("init log routing: %v", err)
	}
	defer func() {
		if cerr := logCloser.Close(); cerr != nil {
			log.Printf("close log routing: %v", cerr)
		}
	}()

	engine, err := db.NewEngine(cfg.Database.Path, cfg.App.Env == "dev")
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

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
	streamer := chooseLLMStreamer(cfg, quality)
	retriever := chooseRetriever(cfg, engine, providerMgr)
	_ = runtime.NewOrchestrator(rt, retriever, streamer, tts, cfg.RAG.TopK, quality)

	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		log.Printf("[audio] vad.start session=%s", evt.SessionID)
	})
	rt.Bus.Subscribe("audio.vad.stop", func(evt bus.Event) {
		log.Printf("[audio] vad.stop session=%s", evt.SessionID)
	})
	rt.Bus.Subscribe("audio.play.start", func(evt bus.Event) {
		log.Printf("[audio] play.start payload=%v", evt.Payload)
	})
	rt.Bus.Subscribe("audio.play.end", func(evt bus.Event) {
		log.Printf("[audio] play.end payload=%v", evt.Payload)
	})
	rt.Bus.Subscribe("error", func(evt bus.Event) {
		log.Printf("[error] payload=%v", evt.Payload)
	})

	rt.Bus.Subscribe("rag.ready", func(evt bus.Event) {
		log.Printf("[demo] rag.ready chunks=%v", evt.Payload)
	})
	rt.Bus.Subscribe("asr.partial", func(evt bus.Event) {
		if s, ok := evt.Payload.(string); ok {
			log.Printf("[demo] asr.partial=%q", s)
		}
	})
	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		if s, ok := evt.Payload.(string); ok {
			log.Printf("[demo] llm.token=%q", s)
		}
	})
	rt.Bus.Subscribe("tts.chunk.ready", func(evt bus.Event) {
		log.Printf("[demo] tts.chunk bytes=%v", evt.Payload)
	})
	rt.Bus.Subscribe("live2d.motion", func(evt bus.Event) {
		log.Printf("[live2d] motion=%v", evt.Payload)
	})
	rt.Bus.Subscribe("live2d.mouth", func(evt bus.Event) {
		log.Printf("[live2d] mouth=%v", evt.Payload)
	})

	log.Printf("[%s] started env=%s version=%s commit=%s db=%s state=%s", cfg.App.Name, cfg.App.Env, version, commit, cfg.Database.Path, rt.State.Get())

	if *demo {
		runDemoInputPipeline(cfg, rt, providerMgr)
		time.Sleep(6 * time.Second)
		log.Printf("[demo] final state=%s degraded=%v", rt.State.Get(), quality.IsDegraded())
		return
	}

	select {}
}

func runDemoInputPipeline(cfg *config.Config, rt *runtime.Runtime, mgr *providers.Manager) {
	sessionID := "demo-session"
	traceID := "demo-trace"

	pipe := audio.Pipeline{
		VAD:           audio.NewEnergyVAD(0.02),
		WakeWord:      chooseWakeWord(cfg, mgr),
		ASR:           chooseASR(cfg, mgr),
		WakeMode:      cfg.Audio.WakeMode,
		SilenceFrames: cfg.Audio.EndpointSilenceFrames,
	}
	pipe.Preprocessors = choosePreprocessors(cfg)
	source := audio.NewMockFrameSource(audio.DefaultDemoFrames(), 40*time.Millisecond)

	go func() {
		if err := pipe.Run(
			context.Background(),
			source,
			func() {
				rt.Bus.Publish(bus.Event{Name: "audio.vad.start", SessionID: sessionID, TraceID: traceID})
				log.Printf("[demo] vad.speech_start")
			},
			func() {
				rt.Bus.Publish(bus.Event{Name: "audio.vad.stop", SessionID: sessionID, TraceID: traceID})
			},
			func(partial string) {
				rt.Bus.Publish(bus.Event{Name: "asr.partial", SessionID: sessionID, TraceID: traceID, Payload: partial})
			},
			func(final string) {
				rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: sessionID, TraceID: traceID, Payload: final})
			},
		); err != nil {
			log.Printf("[demo] audio pipeline failed: %v", err)
		}
	}()
}
func chooseWakeWord(cfg *config.Config, mgr *providers.Manager) audio.WakeWord {
	if strings.ToLower(strings.TrimSpace(cfg.Audio.WakeMode)) != audio.WakeModeWakeWordVAD {
		return nil
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Audio.WakeProvider))
	if provider == "" {
		return &audio.UnavailableWakeWord{Reason: "wake provider is empty"}
	}
	if provider == "sherpa-onnx" {
		modelDir := strings.TrimSpace(cfg.Audio.WakeModelPath)
		if modelDir == "" {
			modelDir = filepath.Join(mgr.Root(), "wake", "current")
		}
		encoder := firstExistingPath(filepath.Join(modelDir, "encoder-epoch-*.onnx"), filepath.Join(modelDir, "encoder.int8.onnx"), filepath.Join(modelDir, "encoder.onnx"))
		decoder := firstExistingPath(filepath.Join(modelDir, "decoder-epoch-*.onnx"), filepath.Join(modelDir, "decoder.int8.onnx"), filepath.Join(modelDir, "decoder.onnx"))
		joiner := firstExistingPath(filepath.Join(modelDir, "joiner-epoch-*.onnx"), filepath.Join(modelDir, "joiner.int8.onnx"), filepath.Join(modelDir, "joiner.onnx"))
		tokens := firstExistingPath(filepath.Join(modelDir, "tokens.txt"))
		keywords := firstExistingPath(strings.TrimSpace(cfg.Audio.WakeKeywordsFilePath), filepath.Join(modelDir, "keywords.txt"), filepath.Join(modelDir, "test_keywords.txt"))
		if encoder == "" || decoder == "" || joiner == "" || tokens == "" || keywords == "" {
			log.Printf("[wake] sherpa-onnx assets incomplete")
			return &audio.UnavailableWakeWord{Reason: "sherpa-onnx assets incomplete"}
		}
		wake := audio.NewSherpaEmbeddedWakeWord(encoder, decoder, joiner, tokens, keywords, cfg.Audio.WakeKeyword)
		_ = framesFromMs(cfg.Audio.WakeDetectIntervalMs)
		log.Printf("[wake] use embedded sherpa-onnx keyword=%s keywords=%s", cfg.Audio.WakeKeyword, keywords)
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
	return []audio.Preprocessor{audio.NewMockAECNS(label)}
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
		model := mgr.ResolveFile("asr", "", cfg.ASR.ModelPath, "ggml-base.bin", "ggml-small.bin", "ggml-medium.bin", "ggml-large-v3.bin")
		if model == "" {
			log.Printf("[asr] whisper model not configured")
			return audio.UnavailableASR{Reason: "whisper model missing"}
		}
		if embedded, err := audio.NewWhisperEmbeddedASR(model, cfg.ASR.Language, cfg.ASR.Threads); err == nil {
			log.Printf("[asr] use embedded whisper model=%s", model)
			return embedded
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
	case "piper":
		binary := mgr.ResolveBinary("tts", "", cfg.TTS.BinaryPath, "piper.exe", "piper")
		model := mgr.ResolveFile("tts", "", cfg.TTS.ModelPath, cfg.TTS.Voice+".onnx", "model.onnx")
		if binary == "" || model == "" {
			log.Printf("[tts] piper not configured")
			return fallback
		}
		primary := audio.NewPiperTTS(binary, model, cfg.TTS.Voice)
		degraded := audio.TTS(nil)
		if quality != nil {
			if degradedVoice := strings.TrimSpace(cfg.Perf.DegradedTTSVoice); degradedVoice != "" && degradedVoice != cfg.TTS.Voice {
				degradedModel := mgr.ResolveFile("tts", "", "", degradedVoice+".onnx")
				if degradedModel != "" {
					log.Printf("[tts] degrade voice available voice=%s model=%s", degradedVoice, degradedModel)
					degraded = audio.NewPiperTTS(binary, degradedModel, degradedVoice)
				}
			}
		}
		log.Printf("[tts] use piper binary=%s model=%s", binary, model)
		if degraded != nil && quality != nil {
			return &audio.RoutedTTS{Primary: primary, Degraded: degraded, Fallback: fallback, UseDegraded: quality.IsDegraded}
		}
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

func framesFromMs(ms int) int {
	if ms <= 0 {
		return 0
	}
	frames := ms / 40
	if frames < 1 {
		return 1
	}
	return frames
}
