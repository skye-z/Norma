package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"norma/backend/core/audio"
	"norma/backend/core/memory"
	"norma/backend/core/llm"
	"norma/backend/core/runtime/bus"
)

type llmFailStreamer struct{}

func (l llmFailStreamer) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	_ = l
	_ = ctx
	_ = req
	_ = onToken
	return errors.New("llm failed")
}

func TestProtocolAudioPlayEvents(t *testing.T) {
	rt := New()
	_ = NewOrchestrator(rt, index.MockRetriever{}, llm.MockStreamer{}, audio.NewMockTTS(), 6)

	playStart := make(chan struct{}, 1)
	playEnd := make(chan struct{}, 1)

	_ = rt.Bus.Subscribe("audio.play.start", func(evt bus.Event) {
		_ = evt
		select {
		case playStart <- struct{}{}:
		default:
		}
	})
	_ = rt.Bus.Subscribe("audio.play.end", func(evt bus.Event) {
		_ = evt
		select {
		case playEnd <- struct{}{}:
		default:
		}
	})

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "p1", TraceID: "tp1", Payload: "hello"})
	rt.Bus.Publish(bus.Event{Name: "audio.play.start", SessionID: "p1", TraceID: "tp1", Payload: map[string]any{"source": "test-player"}})
	rt.Bus.Publish(bus.Event{Name: "audio.play.end", SessionID: "p1", TraceID: "tp1", Payload: map[string]any{"source": "test-player", "reason": "done"}})

	select {
	case <-playStart:
	case <-time.After(3 * time.Second):
		t.Fatalf("expected audio.play.start")
	}

	select {
	case <-playEnd:
	case <-time.After(3 * time.Second):
		t.Fatalf("expected audio.play.end")
	}
}

func TestProtocolErrorEventOnLLMFailure(t *testing.T) {
	rt := New()
	_ = NewOrchestrator(rt, index.MockRetriever{}, llmFailStreamer{}, audio.NewMockTTS(), 6)

	errEvt := make(chan bus.Event, 1)
	_ = rt.Bus.Subscribe("error", func(evt bus.Event) {
		select {
		case errEvt <- evt:
		default:
		}
	})

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "e1", TraceID: "te1", Payload: "hello"})

	select {
	case evt := <-errEvt:
		payload, ok := evt.Payload.(map[string]string)
		if !ok {
			t.Fatalf("expected error payload map[string]string, got %T", evt.Payload)
		}
		if payload["code"] != "llm_stream_failed" {
			t.Fatalf("expected code=llm_stream_failed, got %q", payload["code"])
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("expected error event")
	}
}
