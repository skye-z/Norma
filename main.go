package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"time"

	"norma/internal/audio"
	"norma/internal/bootstrap"
	"norma/internal/config"
	"norma/internal/db"
	"norma/internal/desktop"
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
	log.Printf("[build] embedded whisper: %v", audio.EmbeddedWhisperEnabled())
	log.Printf("[build] embedded sherpa: %v", audio.EmbeddedSherpaEnabled())

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

	stack, err := bootstrap.NewStack(cfg, engine)
	if err != nil {
		log.Fatalf("init runtime stack: %v", err)
	}
	rt := stack.Runtime

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
		log.Printf("[demo] final state=%s degraded=%v", rt.State.Get(), stack.Quality.IsDegraded())
		return
	}

	select {}
}

func runDemoInputPipeline(cfg *config.Config, rt *runtime.Runtime, mgr *providers.Manager) {
	sessionID := "demo-session"
	traceID := "demo-trace"

	pipe := bootstrap.BuildVoicePipeline(cfg, mgr)
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
