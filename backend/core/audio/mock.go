// Package audio 音频处理
package audio

import (
	"context"
	"sync"
	"time"
)

// MockASR 模拟 ASR
type MockASR struct {
	FinalText string
	Partials  []string
	Delay     time.Duration
}

// Stream 模拟流式识别
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

// MockWakeWord 模拟唤醒词
type MockWakeWord struct {
	Hits int
}

// Detect 检测唤醒词
func (m *MockWakeWord) Detect(frame []int16) (bool, error) {
	for _, sample := range frame {
		if sample != 0 {
			m.Hits++
			return true, nil
		}
	}
	return false, nil
}

// MockAECNS 模拟回声消除和降噪
type MockAECNS struct {
	label string
}

// NewMockAECNS 创建模拟 AECNS
func NewMockAECNS(label string) *MockAECNS {
	if label == "" {
		label = "aec_ns"
	}
	return &MockAECNS{label: label}
}

// Process 处理音频帧
func (m *MockAECNS) Process(frame []float32) []float32 {
	out := make([]float32, len(frame))
	copy(out, frame)
	for i := range out {
		out[i] *= 0.92
	}
	return out
}

// Name 获取名称
func (m *MockAECNS) Name() string { return m.label }

// MockTTS 模拟 TTS
type MockTTS struct {
	mu    sync.Mutex
	stops map[string]struct{}
}

// NewMockTTS 创建模拟 TTS
func NewMockTTS() *MockTTS {
	return &MockTTS{stops: make(map[string]struct{})}
}

// Stream 模拟流式合成
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

// Stop 模拟停止
func (m *MockTTS) Stop(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stops[sessionID] = struct{}{}
	return nil
}

// SampleRate 获取采样率
func (m *MockTTS) SampleRate() int { return 22050 }

// Channels 获取声道数
func (m *MockTTS) Channels() int { return 1 }

// Encoding 获取编码格式
func (m *MockTTS) Encoding() string { return "pcm_s16le" }

// SupportsProsodyHints 是否支持韵律提示
func (m *MockTTS) SupportsProsodyHints() bool { return false }
