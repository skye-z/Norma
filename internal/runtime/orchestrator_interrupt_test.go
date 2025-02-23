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

type slowLLM struct {
	canceled atomic.Bool
}

func (s *slowLLM) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	_ = req
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	for i := 0; i < 100; i++ {
		select {
		case <-ctx.Done():
			s.canceled.Store(true)
			return ctx.Err()
		case <-ticker.C:
			onToken("x")
		}
	}
	return nil
}

type noopTTS struct{}

func (n noopTTS) Stream(ctx context.Context, text string, onChunk func([]byte)) error {
	_ = ctx
	_ = text
	onChunk([]byte("a"))
	return nil
}
func (n noopTTS) Stop(sessionID string) error { _ = sessionID; return nil }

func TestInterruptCancelsInFlightSession(t *testing.T) {
	rt := New()
	quality := NewQualityController(true, 1200, 4)
	l := &slowLLM{}
	_ = NewOrchestrator(rt, index.MockRetriever{}, l, noopTTS{}, 4, quality)

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "s1", TraceID: "t1", Payload: "hi"})
	time.Sleep(120 * time.Millisecond)
	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "s1", TraceID: "t1"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if l.canceled.Load() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected llm stream to be canceled after interrupt")
}
