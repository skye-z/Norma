package audio

import (
	"context"
	"errors"
	"math"
	"math/rand"
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

// 先返回“没识别到内容”再返回正常结果的 ASR
type flakyASR struct {
	calls   int
	results []error
	final   []string
}

func (s *flakyASR) Stream(ctx context.Context, p func(string), f func(string)) error { return nil }
func (s *flakyASR) TranscribeFrames(ctx context.Context, frames [][]float32, p func(string), f func(string)) error {
	i := s.calls
	s.calls++
	if i < len(s.results) && s.results[i] != nil {
		return s.results[i]
	}
	s.final = append(s.final, "第二句")
	if f != nil {
		f("第二句")
	}
	return nil
}

func loudSilentFrames(loud, silent int) [][]float32 {
	var out [][]float32
	for i := 0; i < loud; i++ {
		f := make([]float32, 320)
		for j := range f {
			f[j] = 0.5
		}
		out = append(out, f)
	}
	for i := 0; i < silent; i++ {
		out = append(out, make([]float32, 320))
	}
	return out
}

// 回归：一次“没识别到内容”不能让整个收音循环退出，否则之后的语音全部无人处理
func TestPipelineKeepsRunningAfterNoSpeechResult(t *testing.T) {
	asr := &flakyASR{results: []error{ErrNoSpeech}}
	var frames [][]float32
	frames = append(frames, loudSilentFrames(20, 30)...) // 第一句：识别为空
	frames = append(frames, loudSilentFrames(20, 30)...) // 第二句：正常
	pipe := Pipeline{VAD: vad.NewEnergyVAD(0.02), ASR: asr, SilenceFrames: 8}
	err := pipe.Run(context.Background(), &sliceFrameSource{frames: frames}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("pipeline must not fail on ErrNoSpeech, got %v", err)
	}
	if asr.calls != 2 || len(asr.final) != 1 {
		t.Fatalf("second utterance was not processed: calls=%d finals=%v", asr.calls, asr.final)
	}
}

func TestPipelineStillFailsOnRealASRErrors(t *testing.T) {
	asr := &flakyASR{results: []error{errors.New("whisper binary crashed")}}
	pipe := Pipeline{VAD: vad.NewEnergyVAD(0.02), ASR: asr, SilenceFrames: 8}
	err := pipe.Run(context.Background(), &sliceFrameSource{frames: loudSilentFrames(20, 30)}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("a real ASR failure must still be reported")
	}
}

// 回归：真实麦克风电平很低（说话时平均幅度约 0.01）。固定阈值 0.02 会把整句话当静音，
// 唤醒后只能靠强制保持的 75 帧收音，之后说的内容全部丢失
func TestPipelineCapturesWholeQuietUtteranceWithAdaptiveVAD(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	level := func(avg float64, n int) [][]float32 {
		sigma := avg / math.Sqrt(2/math.Pi)
		out := make([][]float32, n)
		for i := range out {
			f := make([]float32, 341)
			for j := range f {
				f[j] = float32(rng.NormFloat64() * sigma)
			}
			out[i] = f
		}
		return out
	}
	var frames [][]float32
	frames = append(frames, level(0.0012, 60)...) // 安静的房间
	frames = append(frames, level(0.010, 200)...) // 一段很长的轻声说话（约 4 秒）
	frames = append(frames, level(0.0012, 80)...) // 说完

	capture := &captureFramesASR{}
	pipe := Pipeline{VAD: vad.NewAdaptiveVAD(), ASR: capture, SilenceFrames: 40}
	if err := pipe.Run(context.Background(), &sliceFrameSource{frames: frames}, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if capture.count < 180 {
		t.Fatalf("only %d of ~200 speech frames reached ASR; quiet speech was cut off", capture.count)
	}

	// 对照：同样的音频用旧的固定阈值，整句话都收不到
	old := &captureFramesASR{}
	pipeOld := Pipeline{VAD: vad.NewEnergyVAD(0.02), ASR: old, SilenceFrames: 40}
	_ = pipeOld.Run(context.Background(), &sliceFrameSource{frames: frames}, nil, nil, nil, nil)
	if old.count > 10 {
		t.Fatalf("premise broken: fixed threshold unexpectedly captured %d frames", old.count)
	}
}

type captureFramesASR struct{ count int }

func (c *captureFramesASR) Stream(ctx context.Context, p func(string), f func(string)) error {
	return nil
}
func (c *captureFramesASR) TranscribeFrames(ctx context.Context, frames [][]float32, p func(string), f func(string)) error {
	c.count += len(frames)
	if f != nil {
		f("x")
	}
	return nil
}
