package audio

import (
    "context"
    "sync"
    "time"
)

// MockASR emits one final sentence after a short delay.
type MockASR struct {
    FinalText string
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

    onPartial("正在识别...")
    onFinal(m.text())
    return nil
}

func (m MockASR) text() string {
    if m.FinalText == "" {
        return "你好诺玛，今天帮我整理一下开发计划"
    }
    return m.FinalText
}

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
            onChunk([]byte(ch + text))
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
