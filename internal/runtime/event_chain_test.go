package runtime

import (
	"context"
	"testing"
	"time"

	"norma/internal/audio"
	"norma/internal/index"
	"norma/internal/llm"
	"norma/internal/runtime/bus"
)

func TestEventChainE2E(t *testing.T) {
	rt := New()
	_ = NewOrchestrator(rt, index.MockRetriever{}, llm.MockStreamer{}, audio.NewMockTTS(), 6)

	done := make(chan struct{}, 1)
	_ = rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		_ = evt
		select {
		case done <- struct{}{}:
		default:
		}
	})

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "s-e2e", TraceID: "t-e2e", Payload: "hello"})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("expected llm.done in e2e chain")
	}

	if st := rt.State.Get(); st != "Listening" {
		t.Fatalf("expected state Listening, got %s", st)
	}
}

func TestOrchestratorCancelByInterrupt(t *testing.T) {
	rt := New()
	_ = NewOrchestrator(rt, index.MockRetriever{}, llm.MockStreamer{}, audio.NewMockTTS(), 6)

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "s-int", TraceID: "t-int", Payload: "hello"})
	time.Sleep(100 * time.Millisecond)
	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "s-int", TraceID: "t-int"})

	if st := rt.State.Get(); st != "Listening" {
		t.Fatalf("expected state Listening after interrupt, got %s", st)
	}
}

var _ = context.Background
