package audio

import (
	"context"
	"testing"
	"time"
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
	pipe := Pipeline{VAD: NewEnergyVAD(0.02), ASR: asr}
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
		VAD:      NewEnergyVAD(0.02),
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
