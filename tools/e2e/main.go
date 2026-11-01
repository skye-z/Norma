// e2e 端到端演示：语音(say) -> Whisper 识别 -> 真实运行时栈(混合检索、DeepSeek、系统合成、播放) -> 回复
// 验证“记住……”写入 Markdown 记忆、同轮之后能答出、重启（重建索引）后仍能答出，并设无记忆对照组
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	asr "norma/backend/core/audio/asr"
	"norma/backend/core/bootstrap"
	"norma/backend/core/db"
	"norma/backend/core/runtime/bus"
	config "norma/backend/utils"
)

type turn struct {
	Scenario   string  `json:"scenario"`
	Spoken     string  `json:"spoken"`
	Recognized string  `json:"recognized"`
	ASRMs      float64 `json:"asr_ms"`
	FirstTokMs float64 `json:"first_token_ms"`
	TotalMs    float64 `json:"total_ms"`
	Reply      string  `json:"reply"`
	Saved      string  `json:"memory_saved,omitempty"`
	Expect     string  `json:"expect_in_reply,omitempty"`
	Pass       *bool   `json:"pass,omitempty"`
}

func main() {
	whisperBin := flag.String("whisper-bin", "", "whisper-cli path")
	whisperModel := flag.String("whisper-model", "", "ggml model path")
	staticDir := flag.String("static-model", "", "Model2Vec dir")
	genieBin := flag.String("genie-bin", "", "python executable or packaged worker for the genie TTS provider")
	genieScript := flag.String("genie-script", "", "worker.py path (empty when -genie-bin is a packaged worker)")
	genieRoot := flag.String("genie-root", "", "dir holding GenieData and CharacterModels")
	out := flag.String("out", "", "json output")
	flag.Parse()
	key := os.Getenv("NORMA_LLM_API_KEY")
	if key == "" || *whisperBin == "" || *whisperModel == "" {
		log.Fatal("need NORMA_LLM_API_KEY, -whisper-bin, -whisper-model")
	}
	work, _ := os.MkdirTemp("", "e2e-")
	defer os.RemoveAll(work)
	mem := filepath.Join(work, "memory.md")

	newCfg := func(memPath string) *config.Config {
		c := &config.Config{}
		c.LLM = config.LLMConfig{Provider: "deepseek", Endpoint: "https://api.deepseek.com", Model: "deepseek-flash", APIKey: key, MaxTokens: 120, Temperature: 0.7}
		c.TTS = config.TTSConfig{Provider: "system", Voice: "Tingting", Speed: 1.2}
		if *genieBin != "" {
			c.TTS = config.TTSConfig{Provider: "genie", BinaryPath: *genieBin, Genie: config.TTSGenieConfig{
				Script:       *genieScript,
				DataDir:      filepath.Join(*genieRoot, "GenieData"),
				CharacterDir: filepath.Join(*genieRoot, "CharacterModels/v2ProPlus/feibi"),
				Language:     "zh",
			}}
		}
		c.RAG = config.RAGConfig{Provider: "hybrid", EmbedProvider: "static", EmbedModelDir: *staticDir, VectorIndexPath: filepath.Join(work, filepath.Base(filepath.Dir(memPath))+"-vec.json"), TopK: 4}
		c.Memory.MarkdownPath = memPath
		return c
	}
	recognizer := asr.NewWhisperCPPASR(*whisperBin, *whisperModel, "zh", 4)

	var results []turn
	runTurn := func(st *bootstrap.Stack, scenario, spoken, expect string) {
		wav := filepath.Join(work, fmt.Sprintf("u%d.wav", len(results)))
		samples := synth(work, wav, "Tingting", spoken)
		t0 := time.Now()
		var text string
		if err := recognizer.TranscribeFrames(context.Background(), [][]float32{samples}, nil, func(s string) { text = s }); err != nil {
			log.Fatalf("asr: %v", err)
		}
		text = asr.PostProcessASRText(text, "")
		tr := turn{Scenario: scenario, Spoken: spoken, Recognized: text, ASRMs: ms(t0, time.Now()), Expect: expect}

		var mu sync.Mutex
		done := make(chan struct{})
		var once sync.Once
		var tAsk, tTok time.Time
		unsub := st.Runtime.Bus.SubscribeAll(func(e bus.Event) {
			mu.Lock()
			defer mu.Unlock()
			switch e.Name {
			case "llm.token":
				if tTok.IsZero() {
					tTok = time.Now()
				}
				if s, ok := e.Payload.(string); ok {
					tr.Reply += s
				}
			case "memory.saved":
				if s, ok := e.Payload.(string); ok {
					tr.Saved = s
				}
			case "session.done", "error":
				once.Do(func() { close(done) })
			}
		})
		_ = unsub
		tAsk = time.Now()
		st.Runtime.Bus.Publish(bus.Event{Name: "asr.final", SessionID: fmt.Sprint(time.Now().UnixNano()), TraceID: scenario, Payload: text})
		select {
		case <-done:
		case <-time.After(60 * time.Second):
			log.Printf("timeout in %s", scenario)
		}
		mu.Lock()
		tr.TotalMs = ms(tAsk, time.Now())
		if !tTok.IsZero() {
			tr.FirstTokMs = ms(tAsk, tTok)
		}
		if expect != "" {
			p := strings.Contains(tr.Reply, expect)
			tr.Pass = &p
		}
		mu.Unlock()
		results = append(results, tr)
		fmt.Printf("[%s]\n  说：%s\n  识别：%s  (%.0f ms)\n  回复：%s\n", scenario, spoken, text, tr.ASRMs, tr.Reply)
		if tr.Saved != "" {
			fmt.Printf("  写入记忆：%s\n", tr.Saved)
		}
		if tr.Pass != nil {
			fmt.Printf("  期望含「%s」：%v\n", expect, *tr.Pass)
		}
	}
	newStack := func(label string, memPath string) *bootstrap.Stack {
		engine, err := db.NewEngine(filepath.Join(work, label+".db"), false)
		if err != nil {
			log.Fatal(err)
		}
		st, err := bootstrap.NewStack(newCfg(memPath), engine)
		if err != nil {
			log.Fatal(err)
		}
		return st
	}

	// A、B：同一进程内写入并追问
	s1 := newStack("run1", mem)
	runTurn(s1, "A-写入", "记住我养的猫叫团子。", "")
	runTurn(s1, "B-同进程追问", "我养的猫叫什么名字？", "团子")
	// C：全新进程与全新数据库，只保留 memory.md，验证索引重建
	s2 := newStack("run2", mem)
	runTurn(s2, "C-重启后追问", "我养的猫叫什么名字？", "团子")
	// D：对照，没有任何记忆的全新环境
	s3 := newStack("run3", filepath.Join(work, "empty", "memory.md"))
	runTurn(s3, "D-无记忆对照", "我养的猫叫什么名字？", "")

	body, _ := os.ReadFile(mem)
	fmt.Printf("\nmemory.md 内容：\n%s\n", body)
	if *out != "" {
		b, _ := json.MarshalIndent(map[string]any{"note": "语音为 macOS say 合成，非真人录音", "turns": results, "memory_md": string(body)}, "", "  ")
		_ = os.WriteFile(*out, b, 0o644)
	}
}

func ms(a, b time.Time) float64 { return float64(b.Sub(a).Microseconds()) / 1000 }

func synth(dir, wav, voice, text string) []float32 {
	aiff := strings.TrimSuffix(wav, ".wav") + ".aiff"
	if err := exec.Command("say", "-v", voice, "-o", aiff, text).Run(); err != nil {
		log.Fatal(err)
	}
	if err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16@16000", "-c", "1", aiff, wav).Run(); err != nil {
		log.Fatal(err)
	}
	b, _ := os.ReadFile(wav)
	b = b[44:]
	out := make([]float32, len(b)/2+16000/2*2)
	for i := 0; i < len(b)/2; i++ {
		out[8000+i] = float32(int16(binary.LittleEndian.Uint16(b[i*2:]))) / 32768
	}
	return out[:8000+len(b)/2+8000]
}
