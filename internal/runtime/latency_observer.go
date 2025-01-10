package runtime

import (
    "log"
    "time"

    "norma/internal/metrics"
    "norma/internal/runtime/bus"
)

// AttachLatencyObserver records and logs ASR/LLM/TTS latency metrics.
func AttachLatencyObserver(rt *Runtime, tracker *metrics.Tracker, quality *QualityController) {
    rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
        tracker.MarkASRFinal(evt.SessionID, evt.TraceID, time.Now())
    })

    rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
        if ms, ok := tracker.MarkLLMFirstToken(evt.SessionID, evt.TraceID, time.Now()); ok {
            log.Printf("[metric] llm_first_token_ms=%d session=%s", ms, evt.SessionID)
            if quality != nil {
                quality.ObserveLLMFirstToken(ms)
                if quality.IsDegraded() {
                    log.Printf("[quality] degraded mode enabled")
                }
            }
        }
    })

    rt.Bus.Subscribe("tts.chunk.ready", func(evt bus.Event) {
        if ms, ok := tracker.MarkTTSFirstChunk(evt.SessionID, evt.TraceID, time.Now()); ok {
            log.Printf("[metric] tts_first_chunk_ms=%d session=%s", ms, evt.SessionID)
        }
    })

    rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
        if lat, ok := tracker.SnapshotAndClear(evt.SessionID, evt.TraceID); ok {
            log.Printf("[metric] summary session=%s asr_to_llm_first_ms=%d asr_to_tts_first_ms=%d",
                evt.SessionID,
                lat.ASRToLLMFirstTokenMs,
                lat.ASRToTTSFirstChunkMs,
            )
        }
    })
}
