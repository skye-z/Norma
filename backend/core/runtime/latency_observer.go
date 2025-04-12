package runtime

import (
	"fmt"
	"log"
	"sync"
	"time"

	"norma/backend/core/runtime/bus"
	"norma/backend/core/runtime/state"
	"norma/backend/utils"
)

// AttachLatencyObserver 附加延迟观察器，用于记录和日志记录ASR/LLM/TTS延迟指标
// 订阅各种事件并跟踪关键性能指标
func AttachLatencyObserver(rt *Runtime, tracker *config.Tracker) {
	// 用于存储打断时刻的时间戳
	var bargeMarks sync.Map

	// VAD开始事件 - 检测是否发生了打断
	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		tracker.RecordValue("audio.vad.start", 1)
		// 如果当前状态为说话中，记录打断标记
		if rt.State.Get() == state.Speaking {
			bargeMarks.Store(evt.SessionID+"|"+evt.TraceID, time.Now())
			log.Printf("[barge] vad.start session=%s trace=%s", evt.SessionID, evt.TraceID)
		}
	})

	// 音频播放开始事件
	rt.Bus.Subscribe("audio.play.start", func(evt bus.Event) { tracker.RecordValue("audio.play.start", 1) })

	// 音频播放结束事件
	rt.Bus.Subscribe("audio.play.end", func(evt bus.Event) { tracker.RecordValue("audio.play.end", 1) })

	// RAG准备就绪事件 - 记录RAG处理时间
	rt.Bus.Subscribe("rag.ready", func(evt bus.Event) {
		tracker.RecordValue("rag.ready", toInt64(evt.Payload))
	})

	// ASR最终结果事件 - 标记ASR完成时间
	rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
		tracker.MarkASRFinal(evt.SessionID, evt.TraceID, time.Now())
	})

	// LLM生成token事件 - 记录首个token生成时间
	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		if ms, ok := tracker.MarkLLMFirstToken(evt.SessionID, evt.TraceID, time.Now()); ok {
			log.Printf("[metric] llm_first_token_ms=%d session=%s trace=%s", ms, evt.SessionID, evt.TraceID)
		}
	})

	// TTS chunk就绪事件 - 记录TTS首个chunk生成时间
	rt.Bus.Subscribe("tts.chunk.ready", func(evt bus.Event) {
		tracker.RecordValue("tts.chunk", toInt64(evt.Payload))
		if ms, ok := tracker.MarkTTSFirstChunk(evt.SessionID, evt.TraceID, time.Now()); ok {
			log.Printf("[metric] tts_first_chunk_ms=%d session=%s trace=%s", ms, evt.SessionID, evt.TraceID)
		}
	})

	// 错误事件 - 记录错误并添加告警
	rt.Bus.Subscribe("error", func(evt bus.Event) {
		tracker.RecordValue("error", 1)
		tracker.AddAlert(fmt.Sprintf("runtime error session=%s trace=%s", evt.SessionID, evt.TraceID))
	})

	// 打断事件 - 计算从打断到处理的时间间隔
	rt.Bus.Subscribe("interrupt", func(evt bus.Event) {
		if start, ok := bargeMarks.Load(evt.SessionID + "|" + evt.TraceID); ok {
			elapsed := time.Since(start.(time.Time)).Milliseconds()
			tracker.RecordValue("barge.interrupt_ms", elapsed)
			log.Printf("[barge] interrupt session=%s trace=%s elapsed_ms=%d", evt.SessionID, evt.TraceID, elapsed)
		}
	})

	// 音频播放刷新事件 - 记录刷新操作的延迟
	rt.Bus.Subscribe("audio.play.flush", func(evt bus.Event) {
		if start, ok := bargeMarks.Load(evt.SessionID + "|" + evt.TraceID); ok {
			elapsed := time.Since(start.(time.Time)).Milliseconds()
			tracker.RecordValue("barge.flush_ms", elapsed)
			log.Printf("[barge] play.flush session=%s trace=%s elapsed_ms=%d payload=%v", evt.SessionID, evt.TraceID, elapsed, evt.Payload)
		}
	})

	// 音频播放结束事件 - 记录播放完成的延迟，并清理打断标记
	rt.Bus.Subscribe("audio.play.end", func(evt bus.Event) {
		if start, ok := bargeMarks.Load(evt.SessionID + "|" + evt.TraceID); ok {
			elapsed := time.Since(start.(time.Time)).Milliseconds()
			tracker.RecordValue("barge.end_ms", elapsed)
			log.Printf("[barge] play.end session=%s trace=%s elapsed_ms=%d payload=%v", evt.SessionID, evt.TraceID, elapsed, evt.Payload)
			bargeMarks.Delete(evt.SessionID + "|" + evt.TraceID)
		}
	})

	// 等待播放完成事件 - 记录等待完成的耗时
	rt.Bus.Subscribe("audio.play.wait.complete", func(evt bus.Event) {
		tracker.RecordValue("audio.play.wait.complete", toInt64Map(evt.Payload, "elapsed_ms"))
	})

	// LLM完成事件 - 输出完整的延迟汇总指标
	rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		if lat, ok := tracker.SnapshotAndClear(evt.SessionID, evt.TraceID); ok {
			log.Printf("[metric] summary session=%s trace=%s asr_to_llm_first_ms=%d asr_to_tts_first_ms=%d",
				evt.SessionID, evt.TraceID, lat.ASRToLLMFirstTokenMs, lat.ASRToTTSFirstChunkMs)
		}
	})
}

// toInt64Map 从map中提取指定键的值并转换为int64类型
func toInt64Map(v any, key string) int64 {
	if payload, ok := v.(map[string]any); ok {
		return toInt64(payload[key])
	}
	return 0
}

// toInt64 将任意类型转换为int64，用于处理不同类型的时间戳或计数值
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
