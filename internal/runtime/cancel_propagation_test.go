package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"norma/internal/index"
	"norma/internal/llm"
	"norma/internal/runtime/bus"
)

type blockingRetriever struct {
	canceled atomic.Bool
}

func (r *blockingRetriever) Search(ctx context.Context, query string, topK int) ([]index.Chunk, error) {
	_ = query
	_ = topK
	<-ctx.Done()
	r.canceled.Store(true)
	return nil, ctx.Err()
}

type instantLLM struct{}

func (l instantLLM) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	_ = ctx
	_ = req
	onToken("ok")
	return nil
}

type blockingTTS struct {
	canceled atomic.Bool
	stopped  atomic.Bool
}

func (t *blockingTTS) Stream(ctx context.Context, text string, onChunk func([]byte)) error {
	_ = text
	onChunk([]byte("head"))
	<-ctx.Done()
	t.canceled.Store(true)
	return ctx.Err()
}

func (t *blockingTTS) Stop(sessionID string) error {
	_ = sessionID
	t.stopped.Store(true)
	return nil
}

func TestCancelPropagationRetriever(t *testing.T) {
	rt := New()
	quality := NewQualityController(true, 1200, 4)
	retriever := &blockingRetriever{}
	tts := &blockingTTS{}
	_ = NewOrchestrator(rt, retriever, instantLLM{}, tts, 6, quality)

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "c1", TraceID: "tc1", Payload: "hello"})
	time.Sleep(100 * time.Millisecond)
	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "c1", TraceID: "tc1"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if retriever.canceled.Load() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected retriever context canceled after interrupt")
}

func TestCancelPropagationTTS(t *testing.T) {
	rt := New()
	quality := NewQualityController(true, 1200, 4)
	tts := &blockingTTS{}
	_ = NewOrchestrator(rt, index.MockRetriever{}, instantLLM{}, tts, 6, quality)

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "c2", TraceID: "tc2", Payload: "hello"})
	time.Sleep(120 * time.Millisecond)
	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "c2", TraceID: "tc2"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if tts.canceled.Load() && tts.stopped.Load() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected tts stream canceled and Stop called after interrupt")
}
