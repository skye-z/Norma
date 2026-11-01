// ttsbench 测 Kokoro 合成的加载时间、逐句耗时与实时率，并按发音人输出 WAV 供试听
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tts "norma/backend/core/audio/tts"
)

var sentences = []string{
	"你好呀，今天过得怎么样？",
	"好，我讲一个很短的：有个人去图书馆借书，管理员问他借什么。",
	"记住了，你养的猫叫团子，以后聊到它，我会记得名字。",
	"听起来不错，说说是什么让今天变得顺利吧。",
	"最近睡得不太好的话，可以试试睡前半小时不看手机。",
	"明天下午三点有个会，我到时候提醒你。",
}

type sample struct {
	Text    string  `json:"text"`
	GenMs   float64 `json:"gen_ms"`
	AudioMs float64 `json:"audio_ms"`
	RTF     float64 `json:"rtf"`
}

func main() {
	model := flag.String("model", "", "kokoro model dir")
	provider := flag.String("provider", "cpu", "cpu | coreml")
	threads := flag.Int("threads", 4, "threads")
	sidList := flag.String("sids", "", "comma separated speaker ids to audition (writes wavs)")
	rounds := flag.Int("rounds", 3, "timing rounds over the sentences")
	speed := flag.Float64("speed", 1.0, "speech speed")
	outDir := flag.String("out-dir", "", "dir for audition wavs")
	out := flag.String("out", "", "json result")
	flag.Parse()
	if *model == "" {
		log.Fatal("-model required")
	}

	engine := tts.NewSherpaKokoroTTS(*model, "", 0, float32(*speed), *threads)
	engine.Provider = *provider
	defer engine.Close()

	gen := func(sid int, text string) ([]byte, time.Duration, error) {
		engine.Sid = sid
		var pcm []byte
		t0 := time.Now()
		err := engine.Stream(context.Background(), text, func(c []byte) { pcm = append(pcm, c...) })
		return pcm, time.Since(t0), err
	}

	// 首次调用包含模型加载
	_, loadDur, err := gen(3, sentences[0])
	if err != nil {
		log.Fatalf("first synth: %v", err)
	}
	fmt.Printf("provider=%s threads=%d  首次(含加载)=%.0f ms\n", *provider, *threads, ms(loadDur))

	var samples []sample
	for r := 0; r < *rounds; r++ {
		for _, s := range sentences {
			pcm, d, err := gen(3, s)
			if err != nil {
				log.Fatal(err)
			}
			audio := float64(len(pcm)/2) / 24000 * 1000
			samples = append(samples, sample{s, ms(d), audio, ms(d) / audio})
		}
	}
	var gsum, asum float64
	for _, x := range samples {
		gsum += x.GenMs
		asum += x.AudioMs
	}
	n := float64(len(samples))
	fmt.Printf("逐句合成：平均 %.0f ms，平均音频 %.0f ms，实时率(耗时/音频) %.3f\n", gsum/n, asum/n, gsum/asum)

	if *sidList != "" && *outDir != "" {
		_ = os.MkdirAll(*outDir, 0o755)
		for _, f := range strings.Split(*sidList, ",") {
			sid, err := strconv.Atoi(strings.TrimSpace(f))
			if err != nil {
				continue
			}
			pcm, _, err := gen(sid, sentences[2]+sentences[3])
			if err != nil {
				log.Printf("sid %d: %v", sid, err)
				continue
			}
			writeWAV(filepath.Join(*outDir, fmt.Sprintf("sid%03d.wav", sid)), pcm, 24000)
		}
	}
	if *out != "" {
		b, _ := json.MarshalIndent(map[string]any{"provider": *provider, "threads": *threads, "first_call_ms": ms(loadDur), "mean_gen_ms": gsum / n, "mean_audio_ms": asum / n, "rtf": gsum / asum, "samples": samples}, "", "  ")
		_ = os.WriteFile(*out, b, 0o644)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func writeWAV(path string, pcm []byte, rate uint32) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(pcm)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], rate)
	binary.LittleEndian.PutUint32(h[28:], rate*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(pcm)))
	f.Write(h)
	f.Write(pcm)
}
