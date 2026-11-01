//go:build darwin && cgo && sherpa_embedded

// waketest 离线评估唤醒词检测：把合成语音按指定音量与帧大小喂给应用里使用的同一个检测器，
// 统计是否触发以及触发时刻相对语音结束的延迟。用法：go run -tags sherpa_embedded ./tools/waketest
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	wake "norma/backend/core/audio/wake"
)

func synth(dir, name, voice, text string) []int16 {
	aiff := filepath.Join(dir, name+".aiff")
	wav := filepath.Join(dir, name+".wav")
	if err := exec.Command("say", "-v", voice, "-o", aiff, text).Run(); err != nil {
		log.Fatal(err)
	}
	if err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16@16000", "-c", "1", aiff, wav).Run(); err != nil {
		log.Fatal(err)
	}
	b, _ := os.ReadFile(wav)
	b = b[44:]
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return out
}

func gain(in []int16, g float64) []int16 {
	out := make([]int16, len(in))
	for i, v := range in {
		out[i] = int16(float64(v) * g)
	}
	return out
}

func main() {
	dir := flag.String("dir", "resource/runtime/providers/wake/current", "wake model dir")
	voice := flag.String("voice", "Tingting", "say voice")
	phraseList := flag.String("phrases", "诺玛,诺玛，今天天气怎么样,你好", "comma separated phrases to test (use ; to separate when phrases contain commas)")
	sep := flag.String("sep", ",", "phrase separator")
	flag.Parse()
	glob := func(p string) string {
		m, _ := filepath.Glob(filepath.Join(*dir, p))
		if len(m) == 0 {
			log.Fatalf("missing %s", p)
		}
		return m[0]
	}
	enc, dec, join := glob("encoder-*.onnx"), glob("decoder-*.onnx"), glob("joiner-*.onnx")
	tokens, kw := filepath.Join(*dir, "tokens.txt"), filepath.Join(*dir, "keywords.txt")
	tmp, _ := os.MkdirTemp("", "waketest-")
	defer os.RemoveAll(tmp)

	phrases := strings.Split(*phraseList, *sep)
	fmt.Printf("%-18s %6s %6s %-10s %s\n", "phrase", "gain", "frame", "result", "latency_after_speech_end")
	for _, ph := range phrases {
		speech := synth(tmp, "p", *voice, ph)
		for _, g := range []float64{1.0, 0.5, 0.25, 0.1} {
			for _, fs := range []int{320, 512, 1024} {
				d := wake.NewSherpaEmbeddedWakeWord(enc, dec, join, tokens, kw, "诺玛")
				// 前后各 2 秒完全静音（数字零）
				sig := append(make([]int16, 32000), gain(speech, g)...)
				speechEnd := len(sig)
				sig = append(sig, make([]int16, 32000)...)
				detectedAt := -1
				t0 := time.Now()
				for i := 0; i+fs <= len(sig); i += fs {
					ok, err := d.Detect(sig[i : i+fs])
					if err != nil {
						log.Fatal(err)
					}
					if ok {
						detectedAt = i + fs
						break
					}
				}
				res, lat := "MISS", "-"
				if detectedAt >= 0 {
					res = "HIT"
					lat = fmt.Sprintf("%+.2fs (audio time)", float64(detectedAt-speechEnd)/16000)
				}
				_ = t0
				fmt.Printf("%-18s %6.2f %6d %-10s %s\n", strings.TrimSpace(ph), g, fs, res, lat)
			}
		}
	}
}
