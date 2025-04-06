package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"norma/internal/audio"
	"norma/internal/companion"
	"norma/internal/config"
	"norma/internal/index"
	"norma/internal/llm"
	"norma/internal/runtime/bus"
	"norma/pkg/layout"
)

// Orchestrator wires ASR/RAG/LLM/TTS with event bus.
type Orchestrator struct {
	rt        *Runtime
	retriever index.Retriever
	llm       llm.Streamer
	tts       audio.TTS
	ragTopK   int
	playback  playbackSink

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

type playbackSink interface {
	Queue(sessionID, traceID string, chunk []byte) error
	Complete(sessionID, traceID string)
	Stop(sessionID, traceID, reason string)
	Drain(ctx context.Context, sessionID string) error
	Wait(ctx context.Context, sessionID string) error
}

func NewOrchestrator(
	rt *Runtime,
	retriever index.Retriever,
	llm llm.Streamer,
	tts audio.TTS,
	ragTopK int,
) *Orchestrator {
	return NewOrchestratorWithPlayback(rt, retriever, llm, tts, ragTopK, nil)
}

func NewOrchestratorWithPlayback(
	rt *Runtime,
	retriever index.Retriever,
	llm llm.Streamer,
	tts audio.TTS,
	ragTopK int,
	playback playbackSink,
) *Orchestrator {
	if ragTopK <= 0 {
		ragTopK = 4
	}
	o := &Orchestrator{
		rt:        rt,
		retriever: retriever,
		llm:       llm,
		tts:       tts,
		ragTopK:   ragTopK,
		playback:  playback,
		cancels:   make(map[string]context.CancelFunc),
	}
	o.bind()
	o.rt.Bus.Subscribe("assistant.runtime.hints", func(evt bus.Event) {
		if payload, ok := evt.Payload.(companion.PersonaRuntimeConfig); ok && payload.KeepMicCaptureAlive {
			log.Printf("[orchestrator] keep mic capture alive requested session=%s trace=%s", evt.SessionID, evt.TraceID)
		}
	})
	return o
}

func (o *Orchestrator) bind() {
	o.rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
		text, _ := evt.Payload.(string)
		go o.handleFinalText(evt.SessionID, evt.TraceID, text)
	})

	o.rt.Bus.Subscribe("interrupt", func(evt bus.Event) {
		o.cancelSession(evt.SessionID)
		if err := o.tts.Stop(evt.SessionID); err != nil {
			log.Printf("[orchestrator] stop tts failed: %v", err)
			o.publishError(evt.SessionID, evt.TraceID, "tts_stop_failed", err)
		}
		if o.playback != nil {
			o.playback.Stop(evt.SessionID, evt.TraceID, "interrupt")
		}
		o.rt.Bus.Publish(bus.Event{Name: "audio.play.interrupt", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: map[string]any{"source": "orchestrator", "reason": "interrupt"}})
	})

	o.rt.Bus.Subscribe("assistant.say", func(evt bus.Event) {
		text, interrupt := extractAssistantSay(evt.Payload)
		if strings.TrimSpace(text) == "" {
			return
		}
		sessionID := strings.TrimSpace(evt.SessionID)
		if sessionID == "" {
			sessionID = fmt.Sprintf("assistant-say-%d", time.Now().UnixMilli())
		}
		traceID := strings.TrimSpace(evt.TraceID)
		if traceID == "" {
			traceID = sessionID
		}
		go o.handleAssistantSay(sessionID, traceID, text, interrupt)
	})
}

func (o *Orchestrator) handleFinalText(sessionID, traceID, text string) {
	ctx, cancel := context.WithCancel(context.Background())
	o.storeCancel(sessionID, cancel)
	defer func() {
		cancel()
		o.deleteCancel(sessionID)
	}()

	topK := o.ragTopK

	var chunks []index.Chunk
	var err error
	chunks, err = o.retriever.Search(ctx, text, topK)
	if err != nil {
		log.Printf("[orchestrator] retriever error: %v", err)
		o.publishError(sessionID, traceID, "retriever_failed", err)
	}

	contextTexts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		contextTexts = append(contextTexts, c.Content)
	}
	log.Printf("[rag] ready session=%s trace=%s chunks=%d", sessionID, traceID, len(chunks))
	o.rt.Bus.Publish(bus.Event{Name: "rag.ready", SessionID: sessionID, TraceID: traceID, Payload: len(chunks)})

	var sb strings.Builder
	req := llm.Request{SessionID: sessionID, Prompt: text, Context: contextTexts, Persona: o.currentPersona()}
	log.Printf("[llm] start session=%s trace=%s prompt=%q context_chunks=%d", sessionID, traceID, text, len(contextTexts))
	if err := o.llm.Stream(ctx, req, func(token string) {
		sb.WriteString(token)
		o.rt.Bus.Publish(bus.Event{Name: "llm.token", SessionID: sessionID, TraceID: traceID, Payload: token})
	}); err != nil {
		log.Printf("[orchestrator] llm stream error: %v", err)
		o.publishError(sessionID, traceID, "llm_stream_failed", err)
		o.rt.Bus.Publish(bus.Event{Name: "llm.done", SessionID: sessionID, TraceID: traceID, Payload: "error"})
		return
	}

	answer := strings.TrimSpace(sb.String())
	if answer == "" {
		answer = "抱歉，我现在没有可用回答。"
	}
	answer = companion.RewritePersona(answer)
	log.Printf("[llm] done session=%s trace=%s answer=%q", sessionID, traceID, answer)
	if err := o.streamAnswer(ctx, sessionID, traceID, answer); err != nil && err != context.Canceled {
		log.Printf("[orchestrator] stream answer error: %v", err)
		o.publishError(sessionID, traceID, "tts_stream_failed", err)
	}
	o.rt.Bus.Publish(bus.Event{Name: "llm.done", SessionID: sessionID, TraceID: traceID})
}

func extractAssistantSay(payload any) (string, bool) {
	data, _ := payload.(map[string]any)
	text := ""
	interrupt := false
	if data != nil {
		if value, ok := data["text"]; ok {
			text = strings.TrimSpace(fmt.Sprint(value))
		}
		if value, ok := data["interrupt"].(bool); ok {
			interrupt = value
		}
		return text, interrupt
	}
	return strings.TrimSpace(fmt.Sprint(payload)), false
}

func (o *Orchestrator) handleAssistantSay(sessionID, traceID, text string, interrupt bool) {
	if interrupt {
		o.stopActiveSessions("assistant_say")
	}
	ctx, cancel := context.WithCancel(context.Background())
	o.storeCancel(sessionID, cancel)
	defer func() {
		cancel()
		o.deleteCancel(sessionID)
	}()
	if err := o.streamAnswer(ctx, sessionID, traceID, strings.TrimSpace(text)); err != nil && err != context.Canceled {
		log.Printf("[orchestrator] assistant.say stream error: %v", err)
		o.publishError(sessionID, traceID, "assistant_say_failed", err)
	}
}

func (o *Orchestrator) currentConfig() *config.Config {
	cfg, err := config.Load(layout.ConfigFileName)
	if err != nil || cfg == nil {
		return nil
	}
	return cfg
}

func (o *Orchestrator) currentPersona() string {
	cfg := o.currentConfig()
	if cfg == nil {
		return config.DefaultLLMPersona
	}
	persona := strings.TrimSpace(cfg.LLM.Persona)
	if persona == "" {
		return config.DefaultLLMPersona
	}
	return persona
}

func (o *Orchestrator) streamAnswer(ctx context.Context, sessionID, traceID, answer string) error {
	cfg := o.currentConfig()
	sentenceFirstEnabled := cfg == nil || cfg.TTS.SentenceFirstEnabled
	plan := companion.PlanReply(answer)
	if !sentenceFirstEnabled {
		cleaned := strings.TrimSpace(answer)
		if cleaned == "" {
			cleaned = "???????????????"
		}
		plan = companion.ReplyPlan{Raw: cleaned, Styled: cleaned, Conclusion: cleaned, Segments: []string{cleaned}}
	}
	if len(plan.Segments) == 0 {
		plan = companion.PlanReply("???????????????")
	}
	emotion := companion.AnalyzeEmotion(answer)
	plan.Delivery = companion.BuildDeliveryPlan(plan, emotion)
	plan.Styled = answer
	runtimeHints := companion.BuildPersonaRuntimeConfig(plan.Delivery, true)
	log.Printf("[tts] runtime config session=%s trace=%s sentence_first_enabled=%v segments=%d", sessionID, traceID, sentenceFirstEnabled, len(plan.Segments))
	o.rt.Bus.Publish(bus.Event{Name: "assistant.reply.plan", SessionID: sessionID, TraceID: traceID, Payload: plan})
	o.rt.Bus.Publish(bus.Event{Name: "assistant.runtime.hints", SessionID: sessionID, TraceID: traceID, Payload: runtimeHints})
	o.rt.Bus.Publish(bus.Event{Name: "live2d.emotion", SessionID: sessionID, TraceID: traceID, Payload: emotion})

	for index, segment := range plan.Segments {
		if err := ctx.Err(); err != nil {
			return err
		}
		phase := "supplement"
		if index == 0 {
			phase = "conclusion"
		}
		delivery := companion.DeliverySpec{Text: segment, Phase: phase, PauseMs: 180, Emphasis: "medium", Energy: emotion.Intensity, Emotion: emotion.Emotion, StyleHint: "steady"}
		if index < len(plan.Delivery) {
			delivery = plan.Delivery[index]
		}
		styledSegment := companion.ApplyDeliveryStyle(segment, delivery, supportsProsodyHints(o.tts))
		log.Printf("[tts] segment start session=%s trace=%s index=%d phase=%s text=%q styled=%q", sessionID, traceID, index, phase, segment, styledSegment)
		o.rt.Bus.Publish(bus.Event{Name: "assistant.reply.segment", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{
			"index":       index,
			"phase":       phase,
			"text":        segment,
			"styled_text": styledSegment,
			"delivery":    delivery,
		}})
		o.emitVisemes(ctx, sessionID, traceID, segment)
		segmentBytes := 0
		sampleRate := 22050
		channels := 1
		encoding := "pcm_s16le"
		if provider, ok := o.tts.(audio.AudioFormatProvider); ok {
			sampleRate = provider.SampleRate()
			channels = provider.Channels()
			encoding = provider.Encoding()
		}
		if err := o.tts.Stream(ctx, styledSegment, func(chunk []byte) {
			segmentBytes += len(chunk)
			if o.playback != nil {
				if err := o.playback.Queue(sessionID, traceID, chunk); err != nil {
					log.Printf("[orchestrator] playback queue error: %v", err)
				}
			}
			o.rt.Bus.Publish(bus.Event{Name: "tts.chunk.ready", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{
				"segment_index": index,
				"bytes":         len(chunk),
				"audio_base64":  encodeBase64(chunk),
				"sample_rate":   sampleRate,
				"channels":      channels,
				"encoding":      encoding,
				"delivery":      delivery,
			}})
		}); err != nil {
			return err
		}
		log.Printf("[tts] segment done session=%s trace=%s index=%d", sessionID, traceID, index)
	}
	if o.playback != nil {
		start := time.Now()
		o.playback.Complete(sessionID, traceID)
		if err := o.playback.Wait(ctx, sessionID); err != nil && err != context.Canceled {
			log.Printf("[orchestrator] playback wait error: %v", err)
		}
		o.rt.Bus.Publish(bus.Event{Name: "audio.play.wait.complete", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"elapsed_ms": time.Since(start).Milliseconds()}})
	}
	o.rt.Bus.Publish(bus.Event{Name: "tts.stream.done", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"source": "orchestrator", "reason": "done"}})
	log.Printf("[tts] stream done session=%s trace=%s", sessionID, traceID)
	return nil
}

func (o *Orchestrator) stopActiveSessions(reason string) {
	o.mu.Lock()
	active := make(map[string]context.CancelFunc, len(o.cancels))
	for sessionID, cancel := range o.cancels {
		active[sessionID] = cancel
	}
	o.mu.Unlock()
	for sessionID, cancel := range active {
		cancel()
		if err := o.tts.Stop(sessionID); err != nil {
			log.Printf("[orchestrator] stop tts failed: %v", err)
		}
		if o.playback != nil {
			o.playback.Stop(sessionID, sessionID, reason)
		}
		o.rt.Bus.Publish(bus.Event{Name: "audio.play.interrupt", SessionID: sessionID, TraceID: sessionID, Payload: map[string]any{"source": "orchestrator", "reason": reason}})
	}
}

func (o *Orchestrator) emitVisemes(ctx context.Context, sessionID, traceID, text string) {
	frames := companion.EstimateVisemes(text)
	go func() {
		for _, frame := range frames {
			select {
			case <-ctx.Done():
				return
			default:
				o.rt.Bus.Publish(bus.Event{Name: "live2d.mouth.viseme", SessionID: sessionID, TraceID: traceID, Payload: frame})
				time.Sleep(time.Duration(frame.Duration) * time.Millisecond)
			}
		}
		o.rt.Bus.Publish(bus.Event{Name: "live2d.mouth.viseme", SessionID: sessionID, TraceID: traceID, Payload: companion.VisemeFrame{Phoneme: "rest", Intensity: 0, Duration: 60}})
	}()
}

func (o *Orchestrator) publishError(sessionID, traceID, code string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	o.rt.Bus.Publish(bus.Event{
		Name:      "error",
		SessionID: sessionID,
		TraceID:   traceID,
		Payload: map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}

func (o *Orchestrator) storeCancel(sessionID string, cancel context.CancelFunc) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if prev, ok := o.cancels[sessionID]; ok {
		prev()
	}
	o.cancels[sessionID] = cancel
}

func (o *Orchestrator) deleteCancel(sessionID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.cancels, sessionID)
}

func (o *Orchestrator) cancelSession(sessionID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if cancel, ok := o.cancels[sessionID]; ok {
		cancel()
		delete(o.cancels, sessionID)
	}
}

func encodeBase64(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func supportsProsodyHints(tts audio.TTS) bool {
	if provider, ok := tts.(audio.ProsodyHintProvider); ok {
		return provider.SupportsProsodyHints()
	}
	return false
}
