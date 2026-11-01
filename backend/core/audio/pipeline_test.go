package audio

import (
	"context"
	"testing"
	"time"

	vad "norma/backend/core/audio/vad"
)

type frameASRStub struct {
	calls      int
	frameCount int
}

func (s *frameASRStub) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	_ = ctx
	if onPartial != nil {
		onPartial("partial")
	}
	if onFinal != nil {
		onFinal("final")
	}
	return nil
}

func (s *frameASRStub) TranscribeFrames(ctx context.Context, frames [][]float32, onPartial func(string), onFinal func(string)) error {
	_ = ctx
	s.calls++
	s.frameCount += len(frames)
	if onPartial != nil {
		onPartial("partial")
	}
	if onFinal != nil {
		onFinal("final")
	}
	return nil
}

func TestPipelineUsesFrameASR(t *testing.T) {
	asr := &frameASRStub{}
	pipe := Pipeline{VAD: vad.NewEnergyVAD(0.02), ASR: asr}
	src := NewMockFrameSource(DefaultDemoFrames(), time.Millisecond)

	finalCount := 0
	err := pipe.Run(context.Background(), src, nil, nil, func(string) {}, func(string) {
		finalCount++
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	if asr.calls == 0 {
		t.Fatalf("expected frame ASR to be called")
	}
	if asr.frameCount == 0 {
		t.Fatalf("expected non-zero frame count")
	}
	if finalCount == 0 {
		t.Fatalf("expected final callback")
	}
}

func TestPipelineSupportsWakeWordMode(t *testing.T) {
	asr := &frameASRStub{}
	wake := &MockWakeWord{}
	pipe := Pipeline{
		VAD:      vad.NewEnergyVAD(0.02),
		WakeWord: wake,
		ASR:      asr,
		WakeMode: WakeModeWakeWordVAD,
	}
	src := NewMockFrameSource(DefaultDemoFrames(), time.Millisecond)
	if err := pipe.Run(context.Background(), src, nil, nil, nil, nil); err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if wake.Hits == 0 {
		t.Fatalf("expected wake word detector to be hit")
	}
	if asr.calls == 0 {
		t.Fatalf("expected ASR after wake word")
	}
}

type sliceFrameSource struct {
	frames [][]float32
	idx    int
}

func (s *sliceFrameSource) Next(ctx context.Context) ([]float32, error) {
	if s.idx >= len(s.frames) {
		return nil, ErrEndOfStream
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	frame := s.frames[s.idx]
	s.idx++
	return frame, nil
}

func TestPipelineFlushesUtteranceOnEndOfStream(t *testing.T) {
	asr := &frameASRStub{}
	pipe := Pipeline{VAD: vad.NewEnergyVAD(0.02), ASR: asr, SilenceFrames: 20}
	speech := make([]float32, 160)
	for i := range speech {
		if i%2 == 0 {
			speech[i] = 0.08
		} else {
			speech[i] = -0.08
		}
	}
	src := &sliceFrameSource{frames: [][]float32{speech, speech, speech, speech}}

	finalCount := 0
	err := pipe.Run(context.Background(), src, nil, nil, func(string) {}, func(string) {
		finalCount++
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if asr.calls != 1 {
		t.Fatalf("expected frame ASR to flush once, got %d", asr.calls)
	}
	if asr.frameCount != 4 {
		t.Fatalf("expected 4 flushed frames, got %d", asr.frameCount)
	}
	if finalCount != 1 {
		t.Fatalf("expected final callback once, got %d", finalCount)
	}
}

func TestPipelineCallsOnWakeOncePerDetectionBeforeSpeechStarts(t *testing.T) {
	var order []string
	pipe := Pipeline{
		VAD:      vad.NewEnergyVAD(0.02),
		WakeWord: &MockWakeWord{},
		ASR:      &frameASRStub{},
		WakeMode: WakeModeWakeWordVAD,
		OnWake:   func() { order = append(order, "wake") },
	}
	src := NewMockFrameSource(DefaultDemoFrames(), time.Millisecond)
	onSpeechStart := func() { order = append(order, "speech") }
	if err := pipe.Run(context.Background(), src, onSpeechStart, nil, nil, nil); err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if len(order) < 2 || order[0] != "wake" {
		t.Fatalf("OnWake must fire first when a wake word is detected, got %v", order)
	}
}

func TestPipelineWithoutOnWakeStillWorks(t *testing.T) {
	pipe := Pipeline{VAD: vad.NewEnergyVAD(0.02), WakeWord: &MockWakeWord{}, ASR: &frameASRStub{}, WakeMode: WakeModeWakeWordVAD}
	if err := pipe.Run(context.Background(), NewMockFrameSource(DefaultDemoFrames(), time.Millisecond), nil, nil, nil, nil); err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
}
