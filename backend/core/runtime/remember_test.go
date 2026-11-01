package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"norma/backend/core/runtime/bus"
)

type fakeMemory struct {
	mu    sync.Mutex
	facts []string
	err   error
}

func (m *fakeMemory) Remember(ctx context.Context, fact string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.facts = append(m.facts, fact)
	return nil
}

func (m *fakeMemory) saved() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.facts...)
}

func runOneTurn(t *testing.T, mem *fakeMemory, text string) []string {
	t.Helper()
	rt := New()
	llmStub := captureLLM{ctxs: make(chan []string, 1)}
	o := NewOrchestrator(rt, fixedRetriever{}, llmStub, newSpyTTS(), 4)
	if mem != nil {
		o.SetMemory(mem)
	}
	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "s1", TraceID: "t1", Payload: text})
	select {
	case ctxs := <-llmStub.ctxs:
		return ctxs
	case <-time.After(3 * time.Second):
		t.Fatal("llm was not called")
		return nil
	}
}

func TestRememberCommandWritesMemoryAndTellsModel(t *testing.T) {
	mem := &fakeMemory{}
	ctxs := runOneTurn(t, mem, "记住我喜欢喝美式咖啡")
	if got := mem.saved(); len(got) != 1 || got[0] != "我喜欢喝美式咖啡" {
		t.Fatalf("saved = %v", got)
	}
	if len(ctxs) == 0 || !strings.Contains(ctxs[0], "写入长期记忆") {
		t.Fatalf("model was not told about the save: %v", ctxs)
	}
}

func TestRememberFailureIsReportedToModel(t *testing.T) {
	mem := &fakeMemory{err: errors.New("disk full")}
	ctxs := runOneTurn(t, mem, "请记住明天要开会")
	if len(ctxs) == 0 || !strings.Contains(ctxs[0], "保存失败") {
		t.Fatalf("failure not reported: %v", ctxs)
	}
}

func TestOrdinaryQuestionDoesNotWriteMemory(t *testing.T) {
	mem := &fakeMemory{}
	runOneTurn(t, mem, "今天天气怎么样")
	if got := mem.saved(); len(got) != 0 {
		t.Fatalf("unexpected writes: %v", got)
	}
}

func TestRememberWithoutMemoryWriterIsPlainQuestion(t *testing.T) {
	ctxs := runOneTurn(t, nil, "记住我喜欢喝美式咖啡")
	for _, c := range ctxs {
		if strings.Contains(c, "系统提示") {
			t.Fatalf("no writer configured but note present: %v", ctxs)
		}
	}
}
