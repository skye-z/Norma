// asrbench 用 macOS say 合成固定语句，叠加合成噪声后送入项目的 Whisper 封装，统计字错率
// 噪声为合成白噪声与人声串扰，不是真实环境录音；用法见 README 或 -h
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	asr "norma/backend/core/audio/asr"
)

type cond struct {
	Name  string  `json:"name"`
	Kind  string  `json:"kind"` // clean | white | babble
	SNRdB float64 `json:"snr_db"`
}

type row struct {
	Cond    string  `json:"cond"`
	Index   int     `json:"index"`
	Ref     string  `json:"ref"`
	Hyp     string  `json:"hyp"`
	CER     float64 `json:"cer"`
	Empty   bool    `json:"empty"`
	Latency float64 `json:"latency_ms"`
}

type summary struct {
	Cond      string  `json:"cond"`
	N         int     `json:"n"`
	MeanCER   float64 `json:"mean_cer"`
	ExactRate float64 `json:"exact_rate"`
	EmptyRate float64 `json:"empty_rate"`
	MeanLatMs float64 `json:"mean_latency_ms"`
}

func main() {
	bin := flag.String("bin", "", "whisper-cli path")
	model := flag.String("model", "", "ggml model path")
	sent := flag.String("sentences", "tools/asrbench/sentences.txt", "sentences file")
	voice := flag.String("voice", "Tingting", "say voice for target speech")
	babbleVoices := flag.String("babble-voices", "Meijia,Sinji", "say voices whose speech is mixed as interference")
	workers := flag.Int("workers", 3, "parallel whisper processes")
	out := flag.String("out", "", "json output")
	flag.Parse()
	if *bin == "" || *model == "" {
		log.Fatal("-bin and -model are required")
	}

	raw, err := os.ReadFile(*sent)
	if err != nil {
		log.Fatal(err)
	}
	var refs []string
	for _, l := range strings.Split(string(raw), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			refs = append(refs, l)
		}
	}
	tmp, _ := os.MkdirTemp("", "asrbench-")
	defer os.RemoveAll(tmp)

	target := make([][]float32, len(refs))
	interf := make([][]float32, len(refs))
	for i, r := range refs {
		target[i] = synth(tmp, fmt.Sprintf("t%d", i), *voice, r)
		// 串扰：两个不同声音各读一句与目标无关的话，叠加成人声背景
		var mixed []float32
		for k, v := range strings.Split(*babbleVoices, ",") {
			part := synth(tmp, fmt.Sprintf("b%d_%d", i, k), v, refs[(i+7+5*k)%len(refs)])
			if len(part) > len(mixed) {
				mixed = append(mixed, make([]float32, len(part)-len(mixed))...)
			}
			for j, x := range part {
				mixed[j] += x
			}
		}
		interf[i] = mixed
	}

	conds := []cond{
		{"clean", "clean", 0},
		{"white-20dB", "white", 20}, {"white-10dB", "white", 10}, {"white-5dB", "white", 5}, {"white-0dB", "white", 0},
		{"babble-10dB", "babble", 10}, {"babble-5dB", "babble", 5}, {"babble-0dB", "babble", 0},
	}

	type job struct {
		c   cond
		idx int
	}
	jobs := make(chan job)
	var mu sync.Mutex
	var rows []row
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a := asr.NewWhisperCPPASR(*bin, *model, "zh", 4)
			for j := range jobs {
				rng := rand.New(rand.NewSource(int64(j.idx*131 + int(j.c.SNRdB)*7 + len(j.c.Kind))))
				sig := mix(target[j.idx], interf[j.idx], j.c, rng)
				t0 := time.Now()
				var hyp string
				_ = a.TranscribeFrames(context.Background(), [][]float32{sig}, nil, func(s string) { hyp = s })
				lat := float64(time.Since(t0).Microseconds()) / 1000
				hyp = asr.PostProcessASRText(hyp, "")
				r := row{Cond: j.c.Name, Index: j.idx, Ref: refs[j.idx], Hyp: hyp, Latency: lat, Empty: strings.TrimSpace(hyp) == ""}
				r.CER = cer(norm(refs[j.idx]), norm(hyp))
				mu.Lock()
				rows = append(rows, r)
				mu.Unlock()
			}
		}()
	}
	for _, c := range conds {
		for i := range refs {
			jobs <- job{c, i}
		}
	}
	close(jobs)
	wg.Wait()

	var sums []summary
	fmt.Printf("%-12s %3s %8s %8s %8s %9s\n", "condition", "n", "CER", "exact", "empty", "lat_ms")
	for _, c := range conds {
		s := summary{Cond: c.Name}
		for _, r := range rows {
			if r.Cond != c.Name {
				continue
			}
			s.N++
			s.MeanCER += r.CER
			s.MeanLatMs += r.Latency
			if r.CER == 0 {
				s.ExactRate++
			}
			if r.Empty {
				s.EmptyRate++
			}
		}
		n := float64(s.N)
		s.MeanCER, s.ExactRate, s.EmptyRate, s.MeanLatMs = s.MeanCER/n, s.ExactRate/n, s.EmptyRate/n, s.MeanLatMs/n
		sums = append(sums, s)
		fmt.Printf("%-12s %3d %8.3f %8.3f %8.3f %9.0f\n", s.Cond, s.N, s.MeanCER, s.ExactRate, s.EmptyRate, s.MeanLatMs)
	}
	if *out != "" {
		b, _ := json.MarshalIndent(map[string]any{
			"note":  "合成语音(macOS say)叠加合成噪声，非真实环境录音",
			"voice": *voice, "babble_voices": *babbleVoices, "model": filepath.Base(*model),
			"summary": sums, "rows": rows,
		}, "", "  ")
		_ = os.WriteFile(*out, b, 0o644)
	}
}

// 合成并转为 16kHz 单声道浮点
func synth(dir, name, voice, text string) []float32 {
	aiff := filepath.Join(dir, name+".aiff")
	wav := filepath.Join(dir, name+".wav")
	if err := exec.Command("say", "-v", voice, "-o", aiff, text).Run(); err != nil {
		log.Fatalf("say: %v", err)
	}
	if err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16@16000", "-c", "1", aiff, wav).Run(); err != nil {
		log.Fatalf("afconvert: %v", err)
	}
	b, err := os.ReadFile(wav)
	if err != nil || len(b) < 44 {
		log.Fatalf("read wav: %v", err)
	}
	b = b[44:] // afconvert 输出标准 44 字节头
	if len(b) < 2000 {
		log.Fatalf("voice %q produced no audio for %q", voice, text)
	}
	out := make([]float32, len(b)/2)
	for i := range out {
		out[i] = float32(int16(binary.LittleEndian.Uint16(b[i*2:]))) / 32768
	}
	return out
}

func rms(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(len(x)))
}

// 在目标语音前后各留 0.5 秒静音，并按信噪比叠加噪声
func mix(sig, interf []float32, c cond, rng *rand.Rand) []float32 {
	pad := 8000
	out := make([]float32, len(sig)+2*pad)
	copy(out[pad:], sig)
	if c.Kind == "clean" {
		return out
	}
	noise := make([]float32, len(out))
	switch c.Kind {
	case "white":
		for i := range noise {
			noise[i] = float32(rng.NormFloat64())
		}
	case "babble":
		for i := range noise {
			noise[i] = interf[i%len(interf)]
		}
	}
	gain := rms(sig) / (rms(noise) * math.Pow(10, c.SNRdB/20))
	for i := range out {
		v := float64(out[i]) + gain*float64(noise[i])
		out[i] = float32(math.Max(-1, math.Min(1, v)))
	}
	return out
}

var digits = strings.NewReplacer("0", "零", "1", "一", "2", "二", "3", "三", "4", "四", "5", "五", "6", "六", "7", "七", "8", "八", "9", "九")

// 去标点与空白，阿拉伯数字转中文数字后再比较
func norm(s string) []rune {
	s = digits.Replace(s)
	var out []rune
	for _, r := range s {
		if unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsSymbol(r) {
			continue
		}
		out = append(out, unicode.ToLower(r))
	}
	return out
}

func cer(ref, hyp []rune) float64 {
	if len(ref) == 0 {
		return 0
	}
	prev := make([]int, len(hyp)+1)
	cur := make([]int, len(hyp)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ref); i++ {
		cur[0] = i
		for j := 1; j <= len(hyp); j++ {
			cost := 1
			if ref[i-1] == hyp[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return float64(prev[len(hyp)]) / float64(len(ref))
}
