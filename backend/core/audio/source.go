// Package audio 音频处理
package audio

import (
	"context"
	"errors"
	"time"
)

var ErrEndOfStream = errors.New("end of stream")

// 提供 PCM float 音频帧
type FrameSource interface {
	Next(ctx context.Context) ([]float32, error)
}

// 模拟帧源，按固定间隔发送预制帧
type MockFrameSource struct {
	frames   [][]float32
	idx      int
	interval time.Duration
}

// 创建模拟帧源
func NewMockFrameSource(frames [][]float32, interval time.Duration) *MockFrameSource {
	if interval <= 0 {
		interval = 40 * time.Millisecond
	}
	return &MockFrameSource{frames: frames, interval: interval}
}

// 获取下一帧
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

// 生成演示用音频帧（语音+静音模式）
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
