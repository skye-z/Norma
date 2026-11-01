//go:build darwin && cgo && sherpa_embedded

// pipetest 用应用相同的管道参数复现“唤醒词之后的连续语句丢失”：
// 合成语音按 21ms 一帧喂给真实的唤醒检测与能量 VAD，把管道交给识别的音频存成 wav 并统计时长
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"norma/backend/core/audio"
	audioVad "norma/backend/core/audio/vad"
	wake "norma/backend/core/audio/wake"
)

type captureASR struct{ utterances [][][]float32 }

func (c *captureASR) Stream(ctx context.Context, p func(string), f func(string)) error { return nil }
func (c *captureASR) TranscribeFrames(ctx context.Context, frames [][]float32, p func(string), f func(string)) error {
	cp := make([][]float32, len(frames))
	for i, fr := range frames {
		cp[i] = append([]float32(nil), fr...)
	}
	c.utterances = append(c.utterances, cp)
	if f != nil {
		f("")
	}
	return nil
}

type sliceSource struct {
	frames [][]float32
	i      int
}

func (s *sliceSource) Next(ctx context.Context) ([]float32, error) {
	if s.i >= len(s.frames) {
		return nil, audio.ErrEndOfStream
	}
	f := s.frames[s.i]
	s.i++
	return f, nil
}

func synth(dir, voice, text string) []float32 {
	aiff, wav := filepath.Join(dir, "s.aiff"), filepath.Join(dir, "s.wav")
	if err := exec.Command("say", "-v", voice, "-o", aiff, text).Run(); err != nil {
		log.Fatal(err)
	}
	if err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16@16000", "-c", "1", aiff, wav).Run(); err != nil {
		log.Fatal(err)
	}
	b, _ := os.ReadFile(wav)
	b = b[44:]
	out := make([]float32, len(b)/2)
	for i := range out {
		out[i] = float32(int16(binary.LittleEndian.Uint16(b[i*2:]))) / 32768
	}
	return out
}

func writeWav(path string, frames [][]float32) {
	var pcm []byte
	for _, f := range frames {
		for _, v := range f {
			x := int16(v * 32767)
			pcm = append(pcm, byte(uint16(x)), byte(uint16(x)>>8))
		}
	}
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(pcm)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], 16000)
	binary.LittleEndian.PutUint32(h[28:], 32000)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(pcm)))
	_ = os.WriteFile(path, append(h, pcm...), 0o644)
}

func main() {
	dir := flag.String("dir", "resource/runtime/providers/wake/current", "wake model dir")
	voice := flag.String("voice", "Tingting", "say voice")
	text := flag.String("text", "诺玛，我叫斯凯", "phrase spoken in one breath")
	out := flag.String("out", "/tmp/pipetest", "output dir")
	frame := flag.Int("frame", 341, "samples per frame at 16kHz (341 = one 1024-sample chunk at 48kHz)")
	hold := flag.Int("buffer", 10, "wake buffer frames (app default 10)")
	flag.Parse()
	_ = os.MkdirAll(*out, 0o755)
	glob := func(p string) string {
		m, _ := filepath.Glob(filepath.Join(*dir, p))
		if len(m) == 0 {
			log.Fatalf("missing %s", p)
		}
		return m[0]
	}
	tmp, _ := os.MkdirTemp("", "pipetest-")
	defer os.RemoveAll(tmp)
	speech := synth(tmp, *voice, *text)

	rng := rand.New(rand.NewSource(1))
	noise := func(n int) []float32 { // 房间底噪，幅度远低于 VAD 阈值
		o := make([]float32, n)
		for i := range o {
			o[i] = float32(rng.NormFloat64()) * 0.002
		}
		return o
	}
	sig := append(noise(16000), speech...)
	speechStart, speechEnd := 16000, len(sig)
	sig = append(sig, noise(16000*3)...)

	var frames [][]float32
	for i := 0; i+*frame <= len(sig); i += *frame {
		frames = append(frames, sig[i:i+*frame])
	}
	asr := &captureASR{}
	pipe := audio.Pipeline{
		VAD:              audioVad.NewEnergyVAD(0.02),
		WakeWord:         wake.NewSherpaEmbeddedWakeWord(glob("encoder-*.onnx"), glob("decoder-*.onnx"), glob("joiner-*.onnx"), filepath.Join(*dir, "tokens.txt"), filepath.Join(*dir, "keywords.txt"), "诺玛"),
		ASR:              asr,
		WakeMode:         audio.WakeModeWakeWordVAD,
		WakeBufferFrames: *hold,
		SilenceFrames:    40,
		AllowBargeIn:     true,
	}
	if err := pipe.Run(context.Background(), &sliceSource{frames: frames}, nil, nil, nil, nil); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("phrase=%q speech=%.2fs (%.2f–%.2fs in stream) frame=%d samples (%.0fms) buffer=%d frames\n",
		*text, float64(speechEnd-speechStart)/16000, float64(speechStart)/16000, float64(speechEnd)/16000, *frame, float64(*frame)/16, *hold)
	if len(asr.utterances) == 0 {
		fmt.Println("RESULT: no utterance delivered to ASR")
		return
	}
	for k, u := range asr.utterances {
		n := 0
		for _, f := range u {
			n += len(f)
		}
		p := filepath.Join(*out, fmt.Sprintf("utt%d.wav", k+1))
		writeWav(p, u)
		fmt.Printf("RESULT utterance %d: %d frames = %.2fs (%s)\n", k+1, len(u), float64(n)/16000, strings.TrimPrefix(p, "/"))
	}
}
