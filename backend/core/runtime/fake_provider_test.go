package runtime

import (
	"testing"
	"time"

	"norma/backend/core/memory"
	"norma/backend/core/runtime/bus"
	"norma/backend/core/testdouble"
)

func TestOrchestratorWithFakeProviders(t *testing.T) {
	rt := New()
	tts := &testdouble.FakeTTS{Chunks: [][]byte{[]byte("a"), []byte("b")}}
	_ = NewOrchestrator(
		rt,
		testdouble.FakeRetriever{Chunks: []memory.Chunk{{Content: "ctx"}}},
		testdouble.FakeLLM{Tokens: []string{"o", "k"}},
		tts,
		4,
	)

	done := make(chan struct{}, 1)
	chunkCount := 0
	_ = rt.Bus.Subscribe("tts.chunk.ready", func(evt bus.Event) {
		_ = evt
		chunkCount++
	})
	_ = rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		select {
		case done <- struct{}{}:
		default:
		}
	})

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "s-fake", TraceID: "t-fake", Payload: "hello"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for llm.done")
	}
	if chunkCount != 2 {
		t.Fatalf("expected 2 tts chunks, got %d", chunkCount)
	}
}

func TestOrchestratorInterruptStopsFakeTTS(t *testing.T) {
	rt := New()
	tts := &testdouble.FakeTTS{}
	_ = NewOrchestrator(rt, testdouble.FakeRetriever{}, testdouble.FakeLLM{}, tts, 4)

	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "s-stop", TraceID: "t-stop", Payload: "barge_in"})
	if tts.StopHits != 1 {
		t.Fatalf("expected fake tts stop to be called once, got %d", tts.StopHits)
	}
}
