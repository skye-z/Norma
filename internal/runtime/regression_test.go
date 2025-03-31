package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"norma/internal/audio"
	"norma/internal/index"
	"norma/internal/llm"
	"norma/internal/runtime/bus"
)

type fixedRetriever struct {
	chunks []index.Chunk
}

func (r fixedRetriever) Search(ctx context.Context, query string, topK int) ([]index.Chunk, error) {
	_ = ctx
	_ = query
	_ = topK
	return r.chunks, nil
}

type captureLLM struct {
	ctxs chan []string
}

func (l captureLLM) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case l.ctxs <- req.Context:
	default:
	}
	onToken("好")
	onToken("的")
	return nil
}

type spyTTS struct {
	mu       sync.Mutex
	chunks   int
	stopHits map[string]int
}

func newSpyTTS() *spyTTS {
	return &spyTTS{stopHits: make(map[string]int)}
}

func (s *spyTTS) Stream(ctx context.Context, text string, onChunk func([]byte)) error {
	_ = text
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			onChunk([]byte("chunk"))
			s.mu.Lock()
			s.chunks++
			s.mu.Unlock()
		}
	}
	return nil
}

func (s *spyTTS) Stop(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopHits[sessionID]++
	return nil
}

func (s *spyTTS) chunkCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chunks
}

func (s *spyTTS) stopCount(sessionID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopHits[sessionID]
}

type cancellableLLM struct {
	started  chan struct{}
	canceled atomic.Bool
}

func (l *cancellableLLM) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	_ = req
	select {
	case l.started <- struct{}{}:
	default:
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			l.canceled.Store(true)
			return ctx.Err()
		case <-ticker.C:
			onToken("x")
		}
	}
}

func TestRegressionVoiceQAFlow(t *testing.T) {
	rt := New()
	tts := newSpyTTS()
	llmSpy := captureLLM{ctxs: make(chan []string, 1)}
	_ = NewOrchestrator(rt, fixedRetriever{}, llmSpy, tts, 6)

	done := make(chan struct{}, 1)
	_ = rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		_ = evt
		select {
		case done <- struct{}{}:
		default:
		}
	})

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "voice-s", TraceID: "voice-t", Payload: "今天天气怎么样"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected llm.done")
	}

	if got := rt.State.Get(); got != "Listening" {
		t.Fatalf("expected Listening, got %s", got)
	}
	if tts.chunkCount() == 0 {
		t.Fatalf("expected tts chunks > 0")
	}
}

func TestRegressionVoiceInterrupt(t *testing.T) {
	rt := New()
	llmSlow := &cancellableLLM{started: make(chan struct{}, 1)}
	tts := newSpyTTS()
	_ = NewOrchestrator(rt, fixedRetriever{}, llmSlow, tts, 6)

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "int-s", TraceID: "int-t", Payload: "请开始长回答"})

	select {
	case <-llmSlow.started:
	case <-time.After(1 * time.Second):
		t.Fatalf("llm did not start")
	}

	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "int-s", TraceID: "int-t"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if llmSlow.canceled.Load() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !llmSlow.canceled.Load() {
		t.Fatalf("expected llm canceled after interrupt")
	}
	if tts.stopCount("int-s") == 0 {
		t.Fatalf("expected tts.Stop to be called for interrupted session")
	}
}

func TestRegressionDocumentQAUsesRetrievedContext(t *testing.T) {
	rt := New()
	llmSpy := captureLLM{ctxs: make(chan []string, 1)}
	_ = NewOrchestrator(
		rt,
		fixedRetriever{chunks: []index.Chunk{{DocID: "doc-1", ChunkID: "c-1", Content: "DOC_FACT: 项目目标是完整产品"}}},
		llmSpy,
		audio.NewMockTTS(),
		6,
	)

	done := make(chan struct{}, 1)
	_ = rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		_ = evt
		select {
		case done <- struct{}{}:
		default:
		}
	})

	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "doc-s", TraceID: "doc-t", Payload: "项目目标是什么"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected llm.done")
	}

	var got []string
	select {
	case got = <-llmSpy.ctxs:
	case <-time.After(1 * time.Second):
		t.Fatalf("expected llm context to be captured")
	}

	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "DOC_FACT") {
		t.Fatalf("expected retrieved doc context injected, got: %q", joined)
	}
}

var (
	_ = errors.New
)
