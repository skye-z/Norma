package runtime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"norma/backend/core/llm"
	"norma/backend/core/runtime/bus"
)

// 记录每次请求带来的历史，并按脚本给出回复
type historyLLM struct {
	mu      sync.Mutex
	history [][]llm.Turn
	replies []string
	n       int
	block   chan struct{} // 非空时，Stream 会等到它被关闭或上下文被取消
}

func (l *historyLLM) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	l.mu.Lock()
	l.history = append(l.history, append([]llm.Turn(nil), req.History...))
	i := l.n
	l.n++
	block := l.block
	l.mu.Unlock()
	if block != nil {
		onToken("{{thinking}}说到一半")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-block:
		}
	}
	reply := "好的。"
	if i < len(l.replies) {
		reply = l.replies[i]
	}
	onToken(reply)
	return nil
}

func (l *historyLLM) seen() [][]llm.Turn {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([][]llm.Turn(nil), l.history...)
}

func askAndWait(t *testing.T, rt *Runtime, sid, text string) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	rt.Bus.Subscribe("session.done", func(e bus.Event) {
		if e.SessionID == sid {
			once.Do(func() { close(done) })
		}
	})
	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: sid, TraceID: sid, Payload: text})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("turn %q did not finish", text)
	}
}

func TestSecondTurnCarriesTheFirstTurnAsHistory(t *testing.T) {
	rt := New()
	l := &historyLLM{replies: []string{"{{smile}}好的，skye。", "你叫 skye。"}}
	NewOrchestrator(rt, fixedRetriever{}, l, newSpyTTS(), 4)
	askAndWait(t, rt, "s1", "我叫skye")
	askAndWait(t, rt, "s2", "我叫什么")

	seen := l.seen()
	if len(seen[0]) != 0 {
		t.Fatalf("first turn must have no history, got %v", seen[0])
	}
	if len(seen[1]) != 1 || seen[1][0].User != "我叫skye" {
		t.Fatalf("second turn must carry the first, got %v", seen[1])
	}
	// Live2D 标识不属于正文，不能进入历史
	if strings.Contains(seen[1][0].Assistant, "{{") || seen[1][0].Assistant != "好的，skye。" {
		t.Fatalf("assistant text in history = %q", seen[1][0].Assistant)
	}
}

func TestHistoryKeepsOnlyTheMostRecentTurns(t *testing.T) {
	rt := New()
	l := &historyLLM{}
	o := NewOrchestrator(rt, fixedRetriever{}, l, newSpyTTS(), 4)
	o.historyMax = 3
	for _, name := range []string{"甲", "乙", "丙", "丁", "戊", "己"} {
		askAndWait(t, rt, "s", name)
	}
	last := l.seen()[5] // 第六次请求带来的是前五轮中最近的三轮：丙、丁、戊
	if len(last) != 3 || last[0].User != "丙" || last[2].User != "戊" {
		t.Fatalf("history window wrong: %v", last)
	}
}

func TestHistoryExpiresAfterIdleTime(t *testing.T) {
	rt := New()
	l := &historyLLM{}
	o := NewOrchestrator(rt, fixedRetriever{}, l, newSpyTTS(), 4)
	clock := time.Now()
	o.now = func() time.Time { return clock }
	askAndWait(t, rt, "s1", "第一句")
	clock = clock.Add(o.historyIdle + time.Minute) // 隔了很久再来，应当是新的对话
	askAndWait(t, rt, "s2", "第二句")
	if got := l.seen()[1]; len(got) != 0 {
		t.Fatalf("stale history must be dropped, got %v", got)
	}
}

func TestInterruptedReplyIsNotRecordedAsHistory(t *testing.T) {
	rt := New()
	l := &historyLLM{block: make(chan struct{})}
	NewOrchestrator(rt, fixedRetriever{}, l, newSpyTTS(), 4)
	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "s1", TraceID: "s1", Payload: "讲个长故事"})
	deadline := time.Now().Add(2 * time.Second)
	for len(l.seen()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "s1", TraceID: "s1"})
	l.mu.Lock()
	l.block = nil // 之后的请求正常返回
	l.mu.Unlock()
	askAndWait(t, rt, "s2", "算了")
	if got := l.seen()[1]; len(got) != 0 {
		t.Fatalf("half-spoken reply must not become history, got %v", got)
	}
}

type fakeProfile struct{ text string }

func (f *fakeProfile) Profile() string { return f.text }

type recordingObserver struct {
	mu    sync.Mutex
	turns [][2]string
}

func (r *recordingObserver) OnTurn(user, assistant string) {
	r.mu.Lock()
	r.turns = append(r.turns, [2]string{user, assistant})
	r.mu.Unlock()
}
func (r *recordingObserver) all() [][2]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]string(nil), r.turns...)
}

type profileCaptureLLM struct {
	mu       sync.Mutex
	profiles []string
}

func (l *profileCaptureLLM) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	l.mu.Lock()
	l.profiles = append(l.profiles, req.Profile)
	l.mu.Unlock()
	onToken("{{smile}}好的。")
	return nil
}

func TestProfileIsReadFreshForEveryTurn(t *testing.T) {
	rt := New()
	l := &profileCaptureLLM{}
	o := NewOrchestrator(rt, fixedRetriever{}, l, newSpyTTS(), 4)
	p := &fakeProfile{}
	o.SetProfile(p)
	askAndWait(t, rt, "s1", "你好呀今天")
	p.text = "- 用户叫 Skye" // 后台整理刚写入了新的画像
	askAndWait(t, rt, "s2", "我叫什么名字")
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.profiles[0] != "" || l.profiles[1] != "- 用户叫 Skye" {
		t.Fatalf("profiles = %q", l.profiles)
	}
}

func TestObserverGetsTheFinishedTurnWithoutLive2DMarks(t *testing.T) {
	rt := New()
	o := NewOrchestrator(rt, fixedRetriever{}, &profileCaptureLLM{}, newSpyTTS(), 4)
	obs := &recordingObserver{}
	o.SetTurnObserver(obs)
	askAndWait(t, rt, "s1", "我叫skye")
	got := obs.all()
	if len(got) != 1 || got[0][0] != "我叫skye" || got[0][1] != "好的。" {
		t.Fatalf("observer turns = %v", got)
	}
}

func TestObserverIsNotToldAboutInterruptedTurns(t *testing.T) {
	rt := New()
	l := &historyLLM{block: make(chan struct{})}
	o := NewOrchestrator(rt, fixedRetriever{}, l, newSpyTTS(), 4)
	obs := &recordingObserver{}
	o.SetTurnObserver(obs)
	rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: "s1", TraceID: "s1", Payload: "讲个长故事"})
	deadline := time.Now().Add(2 * time.Second)
	for len(l.seen()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: "s1", TraceID: "s1"})
	time.Sleep(50 * time.Millisecond)
	if got := obs.all(); len(got) != 0 {
		t.Fatalf("interrupted turn must not be curated: %v", got)
	}
}
