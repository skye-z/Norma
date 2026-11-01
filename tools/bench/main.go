// 会话链路基准：用固定问题驱动 记忆检索 -> 大模型 -> 分句合成 -> 播放，并记录各阶段耗时与打断行为
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"norma/backend/core/audio"
	audioTts "norma/backend/core/audio/tts"
	"norma/backend/core/llm"
	"norma/backend/core/memory"
	"norma/backend/core/runtime"
	"norma/backend/core/runtime/bus"
	config "norma/backend/utils"
)

var prompts = []string{
	"请介绍一下你自己。",
	"今天有点累，你能安慰我一下吗？",
	"给我推荐一个周末放松的方式。",
	"用一句话解释什么是事件总线。",
	"我下午三点要开会，帮我想三个提醒自己的小技巧。",
	"你觉得早睡早起有什么好处？",
	"帮我想一个简短的晨间问候。",
	"如果我想学习 Go 语言，应该从哪里开始？",
	"讲个很短的冷笑话吧。",
	"我今天心情不错，你有什么想说的吗？",
}

type trial struct {
	Index         int     `json:"index"`
	Prompt        string  `json:"prompt"`
	RagMs         float64 `json:"rag_ms"`
	LLMFirstMs    float64 `json:"llm_first_token_ms"`
	FirstSentence float64 `json:"first_sentence_ms"`
	TTSFirstMs    float64 `json:"tts_first_chunk_ms"`
	PlayStartMs   float64 `json:"play_start_ms"`
	TotalMs       float64 `json:"total_ms"`
	Sentences     int     `json:"sentences"`
	Reply         string  `json:"reply"`
	Error         string  `json:"error,omitempty"`

	InterruptDoneUs int64  `json:"interrupt_done_us,omitempty"`
	LateEvents      int    `json:"late_events_after_interrupt,omitempty"`
	StateAfter      string `json:"state_after_interrupt,omitempty"`
}

func ms(t0, t time.Time) float64 { return float64(t.Sub(t0).Microseconds()) / 1000 }

func main() {
	n := flag.Int("n", 10, "trials")
	mode := flag.String("mode", "normal", "normal | interrupt")
	play := flag.Bool("play", false, "play audio with afplay")
	voice := flag.String("voice", "Tingting", "say voice")
	memFile := flag.String("memory", "", "markdown memory file (optional)")
	out := flag.String("out", "", "write json result")
	flag.Parse()

	log.SetOutput(os.Stderr)
	key := os.Getenv("NORMA_LLM_API_KEY")
	if key == "" {
		log.Fatal("NORMA_LLM_API_KEY is empty")
	}
	streamer := llm.NewOllamaStreamer("https://api.deepseek.com", "deepseek-flash", key, 40*time.Second).WithOptions(true, 200, 0.7)
	tts := audioTts.NewSystemSayTTS(*voice, 0)

	var retr memory.Retriever
	if *memFile != "" {
		retr = memory.MarkdownRetriever{Store: memory.NewMarkdownStore(*memFile, 0)}
	} else {
		retr = emptyRetriever{}
	}

	var results []trial
	for i := 0; i < *n; i++ {
		rt := runtime.New()
		var o *runtime.Orchestrator
		if *play {
			pb, err := audio.NewPlatformPlaybackSink(rt.Bus, tts.SampleRate(), tts.Channels(), tts.Encoding())
			if err != nil {
				log.Fatal(err)
			}
			o = runtime.NewOrchestratorWithPlayback(rt, retr, streamer, tts, 4, pb)
		} else {
			o = runtime.NewOrchestrator(rt, retr, streamer, tts, 4)
		}
		o.SetPersona(config.DefaultLLMPersona)

		tr := trial{Index: i + 1, Prompt: prompts[i%len(prompts)]}
		var mu sync.Mutex
		var t0, tRag, tTok, tSent, tTTS, tPlay, tDone, tIntDone time.Time
		finished := make(chan struct{})
		var once sync.Once
		interrupted := false
		late := 0
		reply := ""
		rt.Bus.SubscribeAll(func(e bus.Event) {
			mu.Lock()
			defer mu.Unlock()
			now := time.Now()
			if interrupted && !tIntDone.IsZero() {
				switch e.Name {
				case "llm.token", "tts.chunk", "tts.chunk.ready", "llm.sentence", "audio.play.start":
					late++
				}
			}
			switch e.Name {
			case "rag.ready":
				tRag = now
			case "llm.token":
				if tTok.IsZero() {
					tTok = now
				}
				if s, ok := e.Payload.(string); ok {
					reply += s
				}
			case "llm.sentence":
				tr.Sentences++
				if tSent.IsZero() {
					tSent = now
				}
			case "tts.chunk.ready":
				if tTTS.IsZero() {
					tTTS = now
				}
			case "audio.play.start":
				if tPlay.IsZero() {
					tPlay = now
				}
			case "interrupt.done":
				tIntDone = now
				if us, ok := e.Payload.(int64); ok {
					tr.InterruptDoneUs = us
				}
				tr.StateAfter = string(rt.State.Get())
				once.Do(func() { close(finished) })
			case "session.done":
				tDone = now
				once.Do(func() { close(finished) })
			case "error":
				tr.Error = fmt.Sprint(e.Payload)
				once.Do(func() { close(finished) })
			}
		})

		sid, tid := fmt.Sprintf("b%d", i), fmt.Sprintf("tb%d", i)
		t0 = time.Now()
		rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: sid, TraceID: tid, Payload: tr.Prompt})

		if *mode == "interrupt" {
			// 在首句合成完成后的随机时刻打断，模拟用户插话
			for {
				mu.Lock()
				ready := !tTTS.IsZero()
				mu.Unlock()
				if ready {
					break
				}
				time.Sleep(5 * time.Millisecond)
				if time.Since(t0) > 30*time.Second {
					break
				}
			}
			time.Sleep(time.Duration(100+rand.Intn(900)) * time.Millisecond)
			mu.Lock()
			interrupted = true
			mu.Unlock()
			rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: sid, TraceID: tid, Payload: "barge_in"})
			time.Sleep(1500 * time.Millisecond) // 观察打断后是否还有残留输出
		}

		select {
		case <-finished:
		case <-time.After(60 * time.Second):
			tr.Error = "timeout"
		}
		mu.Lock()
		if !tRag.IsZero() {
			tr.RagMs = ms(t0, tRag)
		}
		if !tTok.IsZero() {
			tr.LLMFirstMs = ms(t0, tTok)
		}
		if !tSent.IsZero() {
			tr.FirstSentence = ms(t0, tSent)
		}
		if !tTTS.IsZero() {
			tr.TTSFirstMs = ms(t0, tTTS)
		}
		if !tPlay.IsZero() {
			tr.PlayStartMs = ms(t0, tPlay)
		}
		if !tDone.IsZero() {
			tr.TotalMs = ms(t0, tDone)
		}
		tr.Reply = reply
		tr.LateEvents = late
		mu.Unlock()
		results = append(results, tr)
		log.Printf("trial %d/%d llm_first=%.0fms tts_first=%.0fms play=%.0fms total=%.0fms int_us=%d late=%d err=%s", tr.Index, *n, tr.LLMFirstMs, tr.TTSFirstMs, tr.PlayStartMs, tr.TotalMs, tr.InterruptDoneUs, tr.LateEvents, tr.Error)
	}

	summary := map[string]any{"mode": *mode, "n": *n, "play": *play, "trials": results}
	summary["stats"] = map[string]any{
		"llm_first_token_ms": stats(results, func(t trial) float64 { return t.LLMFirstMs }),
		"first_sentence_ms":  stats(results, func(t trial) float64 { return t.FirstSentence }),
		"tts_first_chunk_ms": stats(results, func(t trial) float64 { return t.TTSFirstMs }),
		"play_start_ms":      stats(results, func(t trial) float64 { return t.PlayStartMs }),
		"total_ms":           stats(results, func(t trial) float64 { return t.TotalMs }),
		"interrupt_done_ms":  stats(results, func(t trial) float64 { return float64(t.InterruptDoneUs) / 1000 }),
	}
	b, _ := json.MarshalIndent(summary, "", "  ")
	if *out != "" {
		_ = os.WriteFile(*out, b, 0o644)
	}
	fmt.Println(string(b))
}

func stats(ts []trial, f func(trial) float64) map[string]float64 {
	var v []float64
	for _, t := range ts {
		if x := f(t); x > 0 {
			v = append(v, x)
		}
	}
	if len(v) == 0 {
		return nil
	}
	sort.Float64s(v)
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	q := func(p float64) float64 { return v[int(float64(len(v)-1)*p+0.5)] }
	return map[string]float64{"n": float64(len(v)), "mean": sum / float64(len(v)), "median": q(0.5), "p90": q(0.9), "min": v[0], "max": v[len(v)-1]}
}

type emptyRetriever struct{}

func (emptyRetriever) Search(ctx context.Context, q string, k int) ([]memory.Chunk, error) {
	return nil, ctx.Err()
}
