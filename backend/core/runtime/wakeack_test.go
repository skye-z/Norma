package runtime

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"norma/backend/core/runtime/bus"
)

// 只记录合成次数与内容的合成器，实现采样格式接口
type countingTTS struct {
	mu    sync.Mutex
	calls int
	out   []byte
	id    string
}

func (c *countingTTS) Stream(ctx context.Context, text string, on func([]byte)) error {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	on(c.out)
	return nil
}
func (c *countingTTS) Stop(string) error { return nil }
func (c *countingTTS) SampleRate() int   { return 24000 }
func (c *countingTTS) Channels() int     { return 1 }
func (c *countingTTS) Encoding() string  { return "pcm_s16le" }
func (c *countingTTS) Identity() string  { return c.id }
func (c *countingTTS) callCount() int    { c.mu.Lock(); defer c.mu.Unlock(); return c.calls }

type recordingSink struct {
	mu       sync.Mutex
	sessions []string
	chunks   int
	complete int
}

func (r *recordingSink) Queue(sid, tid string, chunk []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = append(r.sessions, sid)
	r.chunks++
	return nil
}
func (r *recordingSink) Complete(sid, tid string)                    { r.mu.Lock(); r.complete++; r.mu.Unlock() }
func (r *recordingSink) Stop(sid, tid, reason string)                {}
func (r *recordingSink) Drain(ctx context.Context, sid string) error { return nil }
func (r *recordingSink) Wait(ctx context.Context, sid string) error  { return nil }
func (r *recordingSink) snapshot() (int, int, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.chunks, r.complete, append([]string(nil), r.sessions...)
}

// 24000Hz 单声道 16 位，约 100ms 的有声数据（全零会被当作静音拒绝）
var ackPCM = buildPCM(24000, 0, 100, 0)

func TestWakeAckSynthesizesOnceAndReusesDiskCache(t *testing.T) {
	dir := t.TempDir()
	tts := &countingTTS{out: ackPCM, id: "v1"}
	a := NewWakeAck(bus.New(), tts, &recordingSink{}, "嗯？", dir)
	if err := a.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Prepare(context.Background()); err != nil { // 已就绪，不重复合成
		t.Fatal(err)
	}
	if tts.callCount() != 1 {
		t.Fatalf("synthesized %d times, want 1", tts.callCount())
	}
	// 模拟应用重启：新实例应直接读缓存，不再调用合成器
	tts2 := &countingTTS{out: ackPCM, id: "v1"}
	b := NewWakeAck(bus.New(), tts2, &recordingSink{}, "嗯？", dir)
	if err := b.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tts2.callCount() != 0 {
		t.Fatalf("cache not reused, synthesized %d times", tts2.callCount())
	}
}

func TestWakeAckCacheInvalidatedWhenVoiceChanges(t *testing.T) {
	dir := t.TempDir()
	first := &countingTTS{out: ackPCM, id: "voice-a"}
	if err := NewWakeAck(bus.New(), first, &recordingSink{}, "嗯？", dir).Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	second := &countingTTS{out: ackPCM, id: "voice-b"}
	if err := NewWakeAck(bus.New(), second, &recordingSink{}, "嗯？", dir).Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if second.callCount() != 1 {
		t.Fatalf("changed voice must resynthesize, calls=%d", second.callCount())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("expected two cache files, got %d", len(entries))
	}
	_ = filepath.Join(dir)
}

func TestWakeAckPlaysOnRequestInOwnSession(t *testing.T) {
	b := bus.New()
	sink := &recordingSink{}
	a := NewWakeAck(b, &countingTTS{out: ackPCM, id: "x"}, sink, "嗯？", t.TempDir())
	if err := a.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	b.Publish(bus.Event{Name: WakeAckEvent, SessionID: "mic-1", TraceID: "t1", Payload: map[string]any{"source": "wake_word"}})
	chunks, complete, sessions := sink.snapshot()
	if chunks != 1 || complete != 1 {
		t.Fatalf("chunks=%d complete=%d", chunks, complete)
	}
	// 必须用自己的会话号，不能占用麦克风会话，否则会干扰同会话回复的停止与打断
	if len(sessions) != 1 || sessions[0] == "mic-1" {
		t.Fatalf("ack must use its own session id, got %v", sessions)
	}
}

func TestWakeAckDoesNotStackOverlappingRequests(t *testing.T) {
	b := bus.New()
	sink := &recordingSink{}
	tts := &countingTTS{out: buildPCM(24000, 0, 1000, 0), id: "x"} // 约 1 秒
	a := NewWakeAck(b, tts, sink, "嗯？", t.TempDir())
	if err := a.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ { // 唤醒词与按钮点击几乎同时到达
		b.Publish(bus.Event{Name: WakeAckEvent, SessionID: "s", Payload: map[string]any{}})
	}
	if chunks, _, _ := sink.snapshot(); chunks != 1 {
		t.Fatalf("overlapping requests played %d times, want 1", chunks)
	}
}

func TestWakeAckIgnoresRequestBeforePreparedAndWhenDisabled(t *testing.T) {
	b := bus.New()
	sink := &recordingSink{}
	NewWakeAck(b, &countingTTS{out: ackPCM, id: "x"}, sink, "嗯？", t.TempDir()) // 未 Prepare
	b.Publish(bus.Event{Name: WakeAckEvent, SessionID: "s"})
	if chunks, _, _ := sink.snapshot(); chunks != 0 {
		t.Fatalf("played before prepared: %d", chunks)
	}

	off := &countingTTS{out: ackPCM, id: "x"}
	a := NewWakeAck(bus.New(), off, sink, "", t.TempDir())
	if err := a.Prepare(context.Background()); err != nil || off.callCount() != 0 {
		t.Fatalf("empty text must not synthesize: err=%v calls=%d", err, off.callCount())
	}
}

func TestWakeAckPlaysAgainAfterPreviousFinishes(t *testing.T) {
	b := bus.New()
	sink := &recordingSink{}
	a := NewWakeAck(b, &countingTTS{out: ackPCM, id: "x"}, sink, "嗯？", t.TempDir()) // 约 100ms
	_ = a.Prepare(context.Background())
	b.Publish(bus.Event{Name: WakeAckEvent, SessionID: "s"})
	time.Sleep(160 * time.Millisecond)
	b.Publish(bus.Event{Name: WakeAckEvent, SessionID: "s"})
	if chunks, _, _ := sink.snapshot(); chunks != 2 {
		t.Fatalf("expected 2 plays, got %d", chunks)
	}
}

// 构造 16 位 PCM：先 silenceMs 毫秒近静音，再 speechMs 毫秒较大的方波，最后 tailMs 毫秒近静音
func buildPCM(rate, silenceMs, speechMs, tailMs int) []byte {
	var out []byte
	put := func(ms int, amp int16) {
		for i := 0; i < rate*ms/1000; i++ {
			v := amp
			if i%2 == 1 {
				v = -amp
			}
			out = append(out, byte(uint16(v)), byte(uint16(v)>>8))
		}
	}
	put(silenceMs, 3)
	put(speechMs, 6000)
	put(tailMs, 3)
	return out
}

func TestTrimSilenceRemovesLeadingGapButKeepsSpeech(t *testing.T) {
	const rate = 32000
	in := buildPCM(rate, 1000, 250, 200) // 与真实情况相同：前面约 1 秒静音
	out := TrimSilencePCM16(in, rate)
	inMs := len(in) / 2 * 1000 / rate
	outMs := len(out) / 2 * 1000 / rate
	if outMs >= inMs/2 {
		t.Fatalf("trim too weak: %dms -> %dms", inMs, outMs)
	}
	if outMs < 250 {
		t.Fatalf("speech was cut: only %dms left", outMs)
	}
	if outMs > 250+40+60+40 { // 语音 + 前留 + 后留 + 一个窗口的误差
		t.Fatalf("too much padding kept: %dms", outMs)
	}
}

func TestTrimSilenceReturnsNilForSilentAudio(t *testing.T) {
	if got := TrimSilencePCM16(buildPCM(32000, 500, 0, 0), 32000); got != nil {
		t.Fatalf("silent audio should yield nil, got %d bytes", len(got))
	}
	if got := TrimSilencePCM16(nil, 32000); len(got) != 0 {
		t.Fatalf("empty input should stay empty")
	}
}

func TestPrepareRejectsAudioThatIsOnlySilence(t *testing.T) {
	silent := &countingTTS{out: buildPCM(24000, 600, 0, 0), id: "s"}
	a := NewWakeAck(bus.New(), silent, &recordingSink{}, "嗯？", t.TempDir())
	if err := a.Prepare(context.Background()); err == nil {
		t.Fatal("expected error for silent synthesis")
	}
}
