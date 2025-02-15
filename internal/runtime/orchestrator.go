package runtime

import (
	"context"
	"log"
	"strings"
	"sync"

	"norma/internal/audio"
	"norma/internal/index"
	"norma/internal/llm"
	"norma/internal/runtime/bus"
)

// Orchestrator wires ASR/RAG/LLM/TTS with event bus.
type Orchestrator struct {
	rt        *Runtime
	retriever index.Retriever
	llm       llm.Streamer
	tts       audio.TTS
	ragTopK   int
	quality   *QualityController

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewOrchestrator(
	rt *Runtime,
	retriever index.Retriever,
	llm llm.Streamer,
	tts audio.TTS,
	ragTopK int,
	quality *QualityController,
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
		quality:   quality,
		cancels:   make(map[string]context.CancelFunc),
	}
	o.bind()
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
		o.rt.Bus.Publish(bus.Event{Name: "audio.play.end", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: "interrupt"})
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
	if o.quality != nil {
		topK = o.quality.RAGTopK(topK)
	}

	chunks, err := o.retriever.Search(ctx, text, topK)
	if err != nil {
		log.Printf("[orchestrator] retriever error: %v", err)
		o.publishError(sessionID, traceID, "retriever_failed", err)
	}

	contextTexts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		contextTexts = append(contextTexts, c.Content)
	}
	o.rt.Bus.Publish(bus.Event{Name: "rag.ready", SessionID: sessionID, TraceID: traceID, Payload: len(chunks)})

	var sb strings.Builder
	req := llm.Request{SessionID: sessionID, Prompt: text, Context: contextTexts}
	if err := o.llm.Stream(ctx, req, func(token string) {
		sb.WriteString(token)
		o.rt.Bus.Publish(bus.Event{Name: "llm.token", SessionID: sessionID, TraceID: traceID, Payload: token})
	}); err != nil {
		log.Printf("[orchestrator] llm stream error: %v", err)
		o.publishError(sessionID, traceID, "llm_stream_failed", err)
		o.rt.Bus.Publish(bus.Event{Name: "llm.done", SessionID: sessionID, TraceID: traceID, Payload: "error"})
		return
	}

	answer := sb.String()
	if answer == "" {
		answer = "抱歉，我现在没有可用回答。"
	}

	o.rt.Bus.Publish(bus.Event{Name: "audio.play.start", SessionID: sessionID, TraceID: traceID, Payload: len(answer)})
	if err := o.tts.Stream(ctx, answer, func(chunk []byte) {
		o.rt.Bus.Publish(bus.Event{Name: "tts.chunk.ready", SessionID: sessionID, TraceID: traceID, Payload: len(chunk)})
	}); err != nil {
		log.Printf("[orchestrator] tts stream error: %v", err)
		o.publishError(sessionID, traceID, "tts_stream_failed", err)
	}
	o.rt.Bus.Publish(bus.Event{Name: "audio.play.end", SessionID: sessionID, TraceID: traceID, Payload: "done"})

	o.rt.Bus.Publish(bus.Event{Name: "llm.done", SessionID: sessionID, TraceID: traceID})
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
