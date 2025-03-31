package runtime

import (
	"fmt"
	"log"
	"sync"
	"time"

	"norma/internal/metrics"
	"norma/internal/runtime/bus"
	"norma/internal/runtime/state"
)

// AttachLatencyObserver records and logs ASR/LLM/TTS latency metrics.
func AttachLatencyObserver(rt *Runtime, tracker *metrics.Tracker) {
	var bargeMarks sync.Map
	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		tracker.RecordValue("audio.vad.start", 1)
		if rt.State.Get() == state.Speaking {
			bargeMarks.Store(evt.SessionID+"|"+evt.TraceID, time.Now())
			log.Printf("[barge] vad.start session=%s trace=%s", evt.SessionID, evt.TraceID)
		}
		// auto-degrade removed
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
	rt.Bus.Subscribe("interrupt", func(evt bus.Event) {
		if start, ok := bargeMarks.Load(evt.SessionID + "|" + evt.TraceID); ok {
			elapsed := time.Since(start.(time.Time)).Milliseconds()
			tracker.RecordValue("barge.interrupt_ms", elapsed)
			log.Printf("[barge] interrupt session=%s trace=%s elapsed_ms=%d", evt.SessionID, evt.TraceID, elapsed)
		}
	})
	rt.Bus.Subscribe("audio.play.flush", func(evt bus.Event) {
		if start, ok := bargeMarks.Load(evt.SessionID + "|" + evt.TraceID); ok {
			elapsed := time.Since(start.(time.Time)).Milliseconds()
			tracker.RecordValue("barge.flush_ms", elapsed)
			log.Printf("[barge] play.flush session=%s trace=%s elapsed_ms=%d payload=%v", evt.SessionID, evt.TraceID, elapsed, evt.Payload)
		}
	})
	rt.Bus.Subscribe("audio.play.end", func(evt bus.Event) {
		if start, ok := bargeMarks.Load(evt.SessionID + "|" + evt.TraceID); ok {
			elapsed := time.Since(start.(time.Time)).Milliseconds()
			tracker.RecordValue("barge.end_ms", elapsed)
			log.Printf("[barge] play.end session=%s trace=%s elapsed_ms=%d payload=%v", evt.SessionID, evt.TraceID, elapsed, evt.Payload)
			bargeMarks.Delete(evt.SessionID + "|" + evt.TraceID)
		}
	})
	rt.Bus.Subscribe("audio.play.wait.complete", func(evt bus.Event) {
		tracker.RecordValue("audio.play.wait.complete", toInt64Map(evt.Payload, "elapsed_ms"))
	})
	rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		if lat, ok := tracker.SnapshotAndClear(evt.SessionID, evt.TraceID); ok {
			log.Printf("[metric] summary session=%s trace=%s asr_to_llm_first_ms=%d asr_to_tts_first_ms=%d",
				evt.SessionID, evt.TraceID, lat.ASRToLLMFirstTokenMs, lat.ASRToTTSFirstChunkMs)
		}
	})
}

func toInt64Map(v any, key string) int64 {
	if payload, ok := v.(map[string]any); ok {
		return toInt64(payload[key])
	}
	return 0
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
