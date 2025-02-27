package main

import (
	"context"
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
	"norma/internal/index"
	"norma/internal/live2d"
	"norma/internal/llm"
	"norma/internal/metrics"
	"norma/internal/ops"
	"norma/internal/runtime"
	"norma/internal/runtime/bus"
)

func main() {
	cfgPath := flag.String("config", "configs/config.yaml", "path to config file")
	demo := flag.Bool("demo", false, "run event-chain demo and exit")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
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
	runtime.AttachLatencyObserver(rt, metrics.NewTracker(), quality)

	_ = live2d.NewBridge(rt.Bus)

	tts := chooseTTS(cfg)
	streamer := chooseLLMStreamer(cfg)
	retriever := chooseRetriever(cfg, engine)
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

	log.Printf("[%s] started env=%s db=%s state=%s", cfg.App.Name, cfg.App.Env, cfg.Database.Path, rt.State.Get())

	if *demo {
		runDemoInputPipeline(cfg, rt)
		time.Sleep(6 * time.Second)
		log.Printf("[demo] final state=%s degraded=%v", rt.State.Get(), quality.IsDegraded())
		return
	}

	select {}
}

func runDemoInputPipeline(cfg *config.Config, rt *runtime.Runtime) {
	sessionID := "demo-session"
	traceID := "demo-trace"

	pipe := audio.Pipeline{
		VAD:           audio.NewEnergyVAD(0.02),
		WakeWord:      chooseWakeWord(cfg),
		ASR:           chooseASR(cfg),
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

func chooseWakeWord(cfg *config.Config) audio.WakeWord {
	if strings.ToLower(strings.TrimSpace(cfg.Audio.WakeMode)) != audio.WakeModeWakeWordVAD {
		return nil
	}
	return &audio.MockWakeWord{}
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

func chooseASR(cfg *config.Config) audio.ASR {
	fallback := audio.MockASR{FinalText: "plan today's development tasks."}
	switch strings.ToLower(strings.TrimSpace(cfg.ASR.Provider)) {
	case "http":
		if cfg.ASR.Endpoint == "" {
			return fallback
		}
		asr := audio.NewHTTPASR(cfg.ASR.Endpoint, time.Duration(cfg.ASR.RequestTimeoutMs)*time.Millisecond)
		asr.StreamEndpoint = strings.TrimSpace(cfg.ASR.StreamEndpoint)
		return asr
	case "whisper", "whispercpp":
		if cfg.ASR.BinaryPath == "" || cfg.ASR.ModelPath == "" {
			log.Printf("[asr] whispercpp not configured, fallback to mock")
			return fallback
		}
		return audio.NewWhisperCPPASR(cfg.ASR.BinaryPath, cfg.ASR.ModelPath, cfg.ASR.Language, cfg.ASR.Threads)
	default:
		return fallback
	}
}
func chooseTTS(cfg *config.Config) audio.TTS {
	fallback := audio.NewMockTTS()
	switch strings.ToLower(strings.TrimSpace(cfg.TTS.Provider)) {
	case "http", "xtts":
		if cfg.TTS.Endpoint == "" {
			return fallback
		}
		return audio.NewXTTSStreamer(cfg.TTS.Endpoint, cfg.TTS.Voice, time.Duration(cfg.TTS.RequestTimeoutMs)*time.Millisecond)
	default:
		return fallback
	}
}
func chooseLLMStreamer(cfg *config.Config) llm.Streamer {
	fallback := llm.MockStreamer{}
	if cfg.LLM.Provider == "ollama" {
		if err := llm.PingOllama(cfg.LLM.Endpoint, 2*time.Second); err == nil {
			model := strings.TrimSpace(cfg.LLM.Model)
			if model == "" {
				discovered, derr := llm.DiscoverFirstLLMModel(cfg.LLM.Endpoint, 2*time.Second)
				if derr != nil {
					log.Printf("[llm] discover model failed, fallback to mock: %v", derr)
					return fallback
				}
				model = discovered
			}

			log.Printf("[llm] use ollama provider endpoint=%s model=%s", cfg.LLM.Endpoint, model)
			return llm.FallbackStreamer{
				Primary:  llm.NewOllamaStreamer(cfg.LLM.Endpoint, model, time.Duration(cfg.LLM.RequestTimeoutMs)*time.Millisecond),
				Fallback: fallback,
			}
		}
		log.Printf("[llm] ollama unavailable, fallback to mock")
	}
	return fallback
}

func chooseRetriever(cfg *config.Config, engine *xorm.Engine) index.Retriever {
	fts := index.NewSQLiteStore(engine)

	vecPath := strings.TrimSpace(cfg.RAG.VectorIndexPath)
	if vecPath == "" {
		vecPath = "data/vector_index.json"
	}
	if dir := filepath.Dir(vecPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("[rag] create vector index dir failed, fallback to text-only retriever: %v", err)
			return index.NewHybridRetriever(nil, fts, nil)
		}
	}

	vec, err := index.NewDiskVectorStore(vecPath)
	if err != nil {
		log.Printf("[rag] init vector store failed, fallback to text-only retriever: %v", err)
		return index.NewHybridRetriever(nil, fts, nil)
	}

	log.Printf("[rag] use hybrid retriever vector=%s", vecPath)
	return index.NewHybridRetriever(vec, fts, nil)
}
