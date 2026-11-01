//go:build darwin && cgo && sherpa_embedded

package audio

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func wakeAssets(t *testing.T) (enc, dec, join, tokens, kw string) {
	t.Helper()
	dir := os.Getenv("NORMA_WAKE_MODEL_DIR")
	if dir == "" {
		dir = "../../../../resource/runtime/providers/wake/current"
	}
	pick := func(p string) string {
		m, _ := filepath.Glob(filepath.Join(dir, p))
		if len(m) == 0 {
			t.Skipf("wake model not installed: %s", p)
		}
		return m[0]
	}
	return pick("encoder-*.onnx"), pick("decoder-*.onnx"), pick("joiner-*.onnx"), pick("tokens.txt"), pick("keywords.txt")
}

func sayPCM16k(t *testing.T, voice, text string) []int16 {
	t.Helper()
	dir := t.TempDir()
	aiff, wav := filepath.Join(dir, "a.aiff"), filepath.Join(dir, "a.wav")
	if err := exec.Command("say", "-v", voice, "-o", aiff, text).Run(); err != nil {
		t.Skipf("say unavailable: %v", err)
	}
	if err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16@16000", "-c", "1", aiff, wav).Run(); err != nil {
		t.Skipf("afconvert unavailable: %v", err)
	}
	b, _ := os.ReadFile(wav)
	b = b[44:]
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return out
}

func feed(d interface{ Detect([]int16) (bool, error) }, sig []int16, frame int) (hit bool, at int) {
	for i := 0; i+frame <= len(sig); i += frame {
		ok, err := d.Detect(sig[i : i+frame])
		if err != nil {
			return false, -1
		}
		if ok {
			return true, i + frame
		}
	}
	return false, -1
}

// 回归：只喊唤醒词、后面是安静时必须触发，且延迟很短。
// 旧实现把安静帧丢掉，模型拿不到关键词之后的音频，会一直等到下一次较响的声音才吐出结果
func TestWakeWordTriggersWhenFollowedBySilenceAndQuietSpeech(t *testing.T) {
	enc, dec, join, tokens, kw := wakeAssets(t)
	speech := sayPCM16k(t, "Tingting", "诺玛")
	for _, gain := range []float64{1.0, 0.25, 0.1} {
		quiet := make([]int16, len(speech))
		for i, v := range speech {
			quiet[i] = int16(float64(v) * gain)
		}
		sig := append(make([]int16, 32000), quiet...)
		speechEnd := len(sig)
		sig = append(sig, make([]int16, 32000)...) // 说完之后是 2 秒完全安静

		d := NewSherpaEmbeddedWakeWord(enc, dec, join, tokens, kw, "诺玛")
		hit, at := feed(d, sig, 512)
		if !hit {
			t.Fatalf("gain=%.2f: wake word not detected when followed by silence", gain)
		}
		if delay := float64(at-speechEnd) / 16000; delay > 1.0 {
			t.Fatalf("gain=%.2f: detected %.2fs after speech end, want < 1s", gain, delay)
		}
	}
}

func TestWakeWordDoesNotTriggerOnUnrelatedSpeech(t *testing.T) {
	enc, dec, join, tokens, kw := wakeAssets(t)
	for _, text := range []string{"你好", "今天天气不错", "请帮我打开音乐", "妈妈"} {
		sig := append(append(make([]int16, 16000), sayPCM16k(t, "Tingting", text)...), make([]int16, 32000)...)
		d := NewSherpaEmbeddedWakeWord(enc, dec, join, tokens, kw, "诺玛")
		if hit, _ := feed(d, sig, 512); hit {
			t.Fatalf("false trigger on %q", text)
		}
	}
}
