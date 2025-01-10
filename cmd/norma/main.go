package main

import (
    "flag"
    "log"
    "time"

    "norma/internal/audio"
    "norma/internal/config"
    "norma/internal/db"
    "norma/internal/index"
    "norma/internal/live2d"
    "norma/internal/llm"
    "norma/internal/metrics"
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

    tts := audio.NewMockTTS()
    streamer := chooseLLMStreamer(cfg)
    _ = runtime.NewOrchestrator(rt, index.MockRetriever{}, streamer, tts, cfg.RAG.TopK, quality)

    rt.Bus.Subscribe("rag.ready", func(evt bus.Event) {
        log.Printf("[demo] rag.ready chunks=%v", evt.Payload)
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
        rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "demo-session", TraceID: "demo-trace", Payload: "帮我规划今天的开发任务"})
        time.Sleep(4 * time.Second)
        log.Printf("[demo] final state=%s degraded=%v", rt.State.Get(), quality.IsDegraded())
        return
    }

    select {}
}

func chooseLLMStreamer(cfg *config.Config) llm.Streamer {
    fallback := llm.MockStreamer{}
    if cfg.LLM.Provider == "ollama" {
        if err := llm.PingOllama(cfg.LLM.Endpoint, 2*time.Second); err == nil {
            log.Printf("[llm] use ollama provider endpoint=%s model=%s", cfg.LLM.Endpoint, cfg.LLM.Model)
            return llm.FallbackStreamer{
                Primary:  llm.NewOllamaStreamer(cfg.LLM.Endpoint, cfg.LLM.Model, time.Duration(cfg.LLM.RequestTimeoutMs)*time.Millisecond),
                Fallback: fallback,
            }
        }
        log.Printf("[llm] ollama unavailable, fallback to mock")
    }
    return fallback
}
