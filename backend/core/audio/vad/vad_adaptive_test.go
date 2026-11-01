package audio

import (
	"math"
	"math/rand"
	"testing"
)

// 生成平均绝对幅度约为 level 的帧（高斯噪声，E|x| = σ·√(2/π)）
func frameAt(rng *rand.Rand, level float64) []float32 {
	sigma := level / math.Sqrt(2/math.Pi)
	f := make([]float32, 341)
	for i := range f {
		f[i] = float32(rng.NormFloat64() * sigma)
	}
	return f
}

func countSpeech(v interface{ Detect([]float32) (bool, error) }, rng *rand.Rand, n int, level float64) int {
	c := 0
	for i := 0; i < n; i++ {
		if ok, _ := v.Detect(frameAt(rng, level)); ok {
			c++
		}
	}
	return c
}

// 回归：实测麦克风说话时平均幅度只有 0.005~0.015，旧的固定阈值 0.02 会把整句话当成静音
func TestAdaptiveVADDetectsQuietSpeechThatFixedThresholdMisses(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	fixed := NewEnergyVAD(0.02)
	adaptive := NewAdaptiveVAD()
	for _, v := range []interface{ Detect([]float32) (bool, error) }{fixed, adaptive} {
		countSpeech(v, rng, 60, 0.0012) // 先是安静的房间
	}
	if got := countSpeech(fixed, rng, 100, 0.010); got != 0 {
		t.Fatalf("premise broken: fixed 0.02 VAD should miss 0.010-level speech, detected %d", got)
	}
	if got := countSpeech(adaptive, rng, 100, 0.010); got < 90 {
		t.Fatalf("adaptive VAD detected only %d/100 speech frames", got)
	}
}

func TestAdaptiveVADStaysQuietOnBackgroundNoise(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	v := NewAdaptiveVAD()
	countSpeech(v, rng, 50, 0.0012)
	if got := countSpeech(v, rng, 500, 0.0012); got > 5 {
		t.Fatalf("background noise misdetected as speech in %d/500 frames", got)
	}
}

func TestAdaptiveVADFollowsNoisyRoomAndStillFindsSpeech(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	v := NewAdaptiveVAD()
	countSpeech(v, rng, 50, 0.002)
	// 房间突然变吵：一开始会被当成说话，但不能一直被当成说话
	countSpeech(v, rng, 800, 0.012)
	if got := countSpeech(v, rng, 200, 0.012); got > 20 {
		t.Fatalf("steady loud room still treated as speech: %d/200", got)
	}
	// 在吵闹的房间里，明显更响的说话声仍然要能检测到
	if got := countSpeech(v, rng, 100, 0.060); got < 90 {
		t.Fatalf("speech over noisy room detected only %d/100", got)
	}
}

func TestAdaptiveVADIgnoresEmptyFrames(t *testing.T) {
	v := NewAdaptiveVAD()
	if ok, err := v.Detect(nil); ok || err != nil {
		t.Fatalf("empty frame: ok=%v err=%v", ok, err)
	}
}
