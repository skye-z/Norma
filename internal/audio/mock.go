package audio

import (
	"context"
	"sync"
	"time"
)

// MockASR emits partial and final sentences after a short delay.
type MockASR struct {
	FinalText string
	Partials  []string
	Delay     time.Duration
}

func (m MockASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	delay := m.Delay
	if delay <= 0 {
		delay = 300 * time.Millisecond
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}

	partials := m.Partials
	if len(partials) == 0 {
		partials = []string{"正在识别...", "正在整理答案..."}
	}
	for _, partial := range partials {
		if onPartial != nil {
			onPartial(partial)
		}
	}
	if onFinal != nil {
		onFinal(m.text())
	}
	return nil
}

func (m MockASR) text() string {
	if m.FinalText == "" {
		return "你好诺玛，今天帮我整理一下开发计划。"
	}
	return m.FinalText
}

// MockWakeWord matches when the frame contains non-zero samples.
type MockWakeWord struct {
	Hits int
}

func (m *MockWakeWord) Detect(frame []int16) (bool, error) {
	for _, sample := range frame {
		if sample != 0 {
			m.Hits++
			return true, nil
		}
	}
	return false, nil
}

// MockAECNS is a lightweight preprocessor stub for scene configuration coverage.
type MockAECNS struct {
	label string
}

func NewMockAECNS(label string) *MockAECNS {
	if label == "" {
		label = "aec_ns"
	}
	return &MockAECNS{label: label}
}

func (m *MockAECNS) Process(frame []float32) []float32 {
	out := make([]float32, len(frame))
	copy(out, frame)
	for i := range out {
		out[i] *= 0.92
	}
	return out
}

func (m *MockAECNS) Name() string { return m.label }

// MockTTS emits fake audio chunks and supports stop.
type MockTTS struct {
	mu    sync.Mutex
	stops map[string]struct{}
}

func NewMockTTS() *MockTTS {
	return &MockTTS{stops: make(map[string]struct{})}
}

func (m *MockTTS) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	chunks := []string{"[audio:head]", "[audio:body]", "[audio:tail]"}
	for _, ch := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if onChunk != nil {
				onChunk([]byte(ch + text))
			}
			time.Sleep(80 * time.Millisecond)
		}
	}
	return nil
}

func (m *MockTTS) Stop(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stops[sessionID] = struct{}{}
	return nil
}

func (m *MockTTS) SampleRate() int { return 22050 }

func (m *MockTTS) Channels() int { return 1 }

func (m *MockTTS) Encoding() string { return "pcm_s16le" }
