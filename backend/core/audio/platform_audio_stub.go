//go:build !windows

package audio

import (
	"context"
	"errors"

	"norma/backend/core/runtime/bus"
)

type PlatformCaptureSource struct{}

func NewPlatformCaptureSource(sampleRate, channels int) (*PlatformCaptureSource, error) {
	_ = sampleRate
	_ = channels
	return nil, errors.New("platform audio capture only implemented on windows")
}

func (s *PlatformCaptureSource) Next(ctx context.Context) ([]float32, error) {
	_ = s
	_ = ctx
	return nil, ErrEndOfStream
}

func (s *PlatformCaptureSource) Close() error {
	_ = s
	return nil
}

func (s *PlatformCaptureSource) EnableBackgroundCapture(sessionID string, enabled bool) {
	_ = s
	_ = sessionID
	_ = enabled
}

type platformPlaybackSink struct{}

func NewPlatformPlaybackSink(b *bus.Bus, sampleRate, channels int, encoding string) (*platformPlaybackSink, error) {
	_ = b
	_ = sampleRate
	_ = channels
	_ = encoding
	return nil, errors.New("platform audio playback only implemented on windows")
}

func (s *platformPlaybackSink) Queue(sessionID, traceID string, chunk []byte) error {
	_ = s
	_ = sessionID
	_ = traceID
	_ = chunk
	return nil
}

func (s *platformPlaybackSink) Complete(sessionID, traceID string) {
	_ = s
	_ = sessionID
	_ = traceID
}

func (s *platformPlaybackSink) Stop(sessionID, traceID, reason string) {
	_ = s
	_ = sessionID
	_ = traceID
	_ = reason
}

func (s *platformPlaybackSink) Wait(ctx context.Context, sessionID string) error {
	_ = s
	_ = ctx
	_ = sessionID
	return nil
}

func (s *platformPlaybackSink) Close() error {
	_ = s
	return nil
}
