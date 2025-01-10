package audio

import (
    "context"
    "errors"
    "time"
)

var ErrEndOfStream = errors.New("end of stream")

// FrameSource provides PCM float frames.
type FrameSource interface {
    Next(ctx context.Context) ([]float32, error)
}

// MockFrameSource emits prepared frames with fixed interval.
type MockFrameSource struct {
    frames    [][]float32
    idx       int
    interval  time.Duration
}

func NewMockFrameSource(frames [][]float32, interval time.Duration) *MockFrameSource {
    if interval <= 0 {
        interval = 40 * time.Millisecond
    }
    return &MockFrameSource{frames: frames, interval: interval}
}

func (m *MockFrameSource) Next(ctx context.Context) ([]float32, error) {
    if m.idx >= len(m.frames) {
        return nil, ErrEndOfStream
    }

    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-time.After(m.interval):
    }

    frame := m.frames[m.idx]
    m.idx++
    return frame, nil
}

// DefaultDemoFrames builds a simple speech-then-silence pattern.
func DefaultDemoFrames() [][]float32 {
    frames := make([][]float32, 0, 40)

    silence := make([]float32, 160)
    speech := make([]float32, 160)
    for i := range speech {
        if i%2 == 0 {
            speech[i] = 0.08
        } else {
            speech[i] = -0.08
        }
    }

    for i := 0; i < 6; i++ {
        frames = append(frames, silence)
    }
    for i := 0; i < 14; i++ {
        frames = append(frames, speech)
    }
    for i := 0; i < 10; i++ {
        frames = append(frames, silence)
    }

    return frames
}
