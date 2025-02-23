package runtime

import (
	"fmt"
	"log"
	"time"

	"norma/internal/metrics"
	"norma/internal/runtime/bus"
)

// AttachLatencyObserver records and logs ASR/LLM/TTS latency metrics.
func AttachLatencyObserver(rt *Runtime, tracker *metrics.Tracker, quality *QualityController) {
	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		tracker.RecordValue("audio.vad.start", 1)
		if quality != nil {
			quality.ObservePressure(tracker.Snapshot().ActiveTraces)
		}
	})
	rt.Bus.Subscribe("audio.play.start", func(evt bus.Event) { tracker.RecordValue("audio.play.start", 1) })
	rt.Bus.Subscribe("audio.play.end", func(evt bus.Event) { tracker.RecordValue("audio.play.end", 1) })
	rt.Bus.Subscribe("rag.ready", func(evt bus.Event) {
		tracker.RecordValue("rag.ready", toInt64(evt.Payload))
	})
	rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
		tracker.MarkASRFinal(evt.SessionID, evt.TraceID, time.Now())
	})
	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		if ms, ok := tracker.MarkLLMFirstToken(evt.SessionID, evt.TraceID, time.Now()); ok {
			log.Printf("[metric] llm_first_token_ms=%d session=%s trace=%s", ms, evt.SessionID, evt.TraceID)
			if quality != nil {
				quality.ObserveLLMFirstToken(ms)
				if quality.IsDegraded() {
					log.Printf("[quality] degraded mode enabled")
				}
			}
		}
	})
	rt.Bus.Subscribe("tts.chunk.ready", func(evt bus.Event) {
		tracker.RecordValue("tts.chunk", toInt64(evt.Payload))
		if ms, ok := tracker.MarkTTSFirstChunk(evt.SessionID, evt.TraceID, time.Now()); ok {
			log.Printf("[metric] tts_first_chunk_ms=%d session=%s trace=%s", ms, evt.SessionID, evt.TraceID)
		}
	})
	rt.Bus.Subscribe("error", func(evt bus.Event) {
		tracker.RecordValue("error", 1)
		tracker.AddAlert(fmt.Sprintf("runtime error session=%s trace=%s", evt.SessionID, evt.TraceID))
	})
	rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		if lat, ok := tracker.SnapshotAndClear(evt.SessionID, evt.TraceID); ok {
			log.Printf("[metric] summary session=%s trace=%s asr_to_llm_first_ms=%d asr_to_tts_first_ms=%d",
				evt.SessionID, evt.TraceID, lat.ASRToLLMFirstTokenMs, lat.ASRToTTSFirstChunkMs)
		}
	})
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	default:
		return 0
	}
}
