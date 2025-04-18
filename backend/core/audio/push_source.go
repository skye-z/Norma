// Package audio 音频处理
package audio

import (
	"context"
	"sync"
)

// PushFrameSource 推送帧源
type PushFrameSource struct {
	frames chan []float32
	done   chan struct{}
	once   sync.Once
}

// NewPushFrameSource 创建推送帧源
func NewPushFrameSource(buffer int) *PushFrameSource {
	if buffer <= 0 {
		buffer = 32
	}
	return &PushFrameSource{
		frames: make(chan []float32, buffer),
		done:   make(chan struct{}),
	}
}

// Next 获取下一帧
func (s *PushFrameSource) Next(ctx context.Context) ([]float32, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrEndOfStream
	case frame, ok := <-s.frames:
		if !ok {
			return nil, ErrEndOfStream
		}
		return frame, nil
	}
}

// Push 推送帧
func (s *PushFrameSource) Push(frame []float32) bool {
	if len(frame) == 0 {
		return false
	}
	cpy := make([]float32, len(frame))
	copy(cpy, frame)
	select {
	case <-s.done:
		return false
	case s.frames <- cpy:
		return true
	default:
		return false
	}
}

// Close 关闭
func (s *PushFrameSource) Close() {
	s.once.Do(func() {
		close(s.done)
		close(s.frames)
	})
}
