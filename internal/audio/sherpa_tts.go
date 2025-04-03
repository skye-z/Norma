package audio

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-windows"
)

type SherpaKokoroTTS struct {
	ModelDir string
	Voice    string
	Sid      int
	Speed    float32
	Threads  int

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewSherpaKokoroTTS(modelDir, voice string, sid int, speed float32, threads int) *SherpaKokoroTTS {
	if threads <= 0 {
		threads = 1
	}
	if speed <= 0 {
		speed = 1.0
	}
	return &SherpaKokoroTTS{
		ModelDir: strings.TrimSpace(modelDir),
		Voice:    strings.TrimSpace(voice),
		Sid:      sid,
		Speed:    speed,
		Threads:  threads,
	}
}

func (s *SherpaKokoroTTS) SampleRate() int { return 24000 }

func (s *SherpaKokoroTTS) Channels() int { return 1 }

func (s *SherpaKokoroTTS) Encoding() string { return "pcm_s16le" }

func (s *SherpaKokoroTTS) SupportsProsodyHints() bool { return false }

func sanitizeKokoroInput(text string) string {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return ""
	}
	cleaned = strings.NewReplacer(
		"\u2019", "'",
		"\u2018", "'",
		"\u201c", "\"",
		"\u201d", "\"",
		"\u2014", "-",
		"\u2013", "-",
		"\u2026", "...",
		"\u00a0", " ",
		"\ufffd", "",
	).Replace(cleaned)

	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		switch {
		case r == '\u025a':
			continue
		case r == '\ufffd':
			continue
		case unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r':
			continue
		default:
			b.WriteRune(r)
		}
	}
	cleaned = strings.TrimSpace(b.String())
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if utf8.RuneCountInString(cleaned) == 0 {
		return ""
	}
	return cleaned
}

func (s *SherpaKokoroTTS) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	ensureSherpaDLLPath()
	if s.ModelDir == "" {
		return fmt.Errorf("kokoro model dir empty")
	}
	trimmed := sanitizeKokoroInput(text)
	if trimmed == "" {
		return nil
	}
	if trimmed != strings.TrimSpace(text) {
		log.Printf("[tts] kokoro sanitized input raw=%q sanitized=%q", text, trimmed)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
		cancel()
	}()

	config, err := buildKokoroConfig(s.ModelDir, s.Threads)
	if err != nil {
		return err
	}
	if s.Voice != "" {
		if sid := ResolveSpeakerID(s.Voice); sid >= 0 {
			s.Sid = sid
		}
	}
	if s.Sid < 0 {
		s.Sid = 0
	}
	if s.Speed <= 0 {
		s.Speed = 1.0
	}
	config.MaxNumSentences = 1

	log.Printf("[tts] kokoro ready model_dir=%s sid=%d speed=%.2f", s.ModelDir, s.Sid, s.Speed)
	engine := sherpa.NewOfflineTts(&config)
	if engine == nil {
		return fmt.Errorf("create sherpa offline tts failed")
	}
	defer sherpa.DeleteOfflineTts(engine)

	select {
	case <-streamCtx.Done():
		return streamCtx.Err()
	default:
	}
	audio := engine.Generate(trimmed, s.Sid, float32(math.Max(float64(s.Speed), 1e-6)))
	if audio == nil || len(audio.Samples) == 0 {
		return fmt.Errorf("kokoro generated empty audio")
	}
	if audio.SampleRate > 0 && audio.SampleRate != s.SampleRate() {
		log.Printf("[tts] kokoro sample rate mismatch got=%d expected=%d", audio.SampleRate, s.SampleRate())
	}

	if onChunk != nil {
		chunk := encodePCM16LE(audio.Samples)
		if len(chunk) > 0 {
			onChunk(chunk)
		}
	}
	return nil
}

func (s *SherpaKokoroTTS) Stop(sessionID string) error {
	_ = sessionID
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func buildKokoroConfig(dir string, threads int) (sherpa.OfflineTtsConfig, error) {
	base := filepath.Clean(dir)
	config := sherpa.OfflineTtsConfig{}
	config.Model.Kokoro.Model = filepath.Join(base, "model.onnx")
	config.Model.Kokoro.Voices = filepath.Join(base, "voices.bin")
	config.Model.Kokoro.Tokens = filepath.Join(base, "tokens.txt")
	if _, err := os.Stat(config.Model.Kokoro.Model); err != nil {
		return config, fmt.Errorf("kokoro model not found: %s", config.Model.Kokoro.Model)
	}
	if _, err := os.Stat(config.Model.Kokoro.Voices); err != nil {
		return config, fmt.Errorf("kokoro voices not found: %s", config.Model.Kokoro.Voices)
	}
	if _, err := os.Stat(config.Model.Kokoro.Tokens); err != nil {
		return config, fmt.Errorf("kokoro tokens not found: %s", config.Model.Kokoro.Tokens)
	}
	config.Model.Kokoro.DataDir = filepath.Join(base, "espeak-ng-data")
	if _, err := os.Stat(config.Model.Kokoro.DataDir); err != nil {
		config.Model.Kokoro.DataDir = ""
	}
	config.Model.Kokoro.Lexicon = joinExisting(",",
		filepath.Join(base, "lexicon-us-en.txt"),
		filepath.Join(base, "lexicon-zh.txt"),
	)
	config.RuleFsts = joinExisting(",",
		filepath.Join(base, "date-zh.fst"),
		filepath.Join(base, "number-zh.fst"),
		filepath.Join(base, "phone-zh.fst"),
	)
	if threads <= 0 {
		threads = 1
	}
	config.Model.NumThreads = threads
	config.Model.Provider = "cpu"
	config.MaxNumSentences = 1
	return config, nil
}

func joinExisting(sep string, paths ...string) string {
	items := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			items = append(items, path)
		}
	}
	return strings.Join(items, sep)
}

func encodePCM16LE(samples []float32) []byte {
	if len(samples) == 0 {
		return nil
	}
	out := make([]byte, len(samples)*2)
	for i, v := range samples {
		if v > 1.0 {
			v = 1.0
		} else if v < -1.0 {
			v = -1.0
		}
		iv := int16(v * 32767)
		idx := i * 2
		out[idx] = byte(iv)
		out[idx+1] = byte(iv >> 8)
	}
	return out
}

func ResolveSpeakerID(name string) int {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "af_alloy":
		return 0
	case "af_aoede":
		return 1
	case "af_bella":
		return 2
	case "af_heart":
		return 3
	case "af_jessica":
		return 4
	case "af_kore":
		return 5
	case "af_nicole":
		return 6
	case "af_nova":
		return 7
	case "af_river":
		return 8
	case "af_sarah":
		return 9
	case "af_sky":
		return 10
	case "am_adam":
		return 11
	case "am_echo":
		return 12
	case "am_eric":
		return 13
	case "am_fenrir":
		return 14
	case "am_liam":
		return 15
	case "am_michael":
		return 16
	case "am_onyx":
		return 17
	case "am_puck":
		return 18
	case "am_santa":
		return 19
	case "bf_alice":
		return 20
	case "bf_emma":
		return 21
	case "bf_isabella":
		return 22
	case "bf_lily":
		return 23
	case "bm_daniel":
		return 24
	case "bm_fable":
		return 25
	case "bm_george":
		return 26
	case "bm_lewis":
		return 27
	case "ef_dora":
		return 28
	case "em_alex":
		return 29
	case "ff_siwis":
		return 30
	case "hf_alpha":
		return 31
	case "hf_beta":
		return 32
	case "hm_omega":
		return 33
	case "hm_psi":
		return 34
	case "if_sara":
		return 35
	case "im_nicola":
		return 36
	case "jf_alpha":
		return 37
	case "jf_gongitsune":
		return 38
	case "jf_nezumi":
		return 39
	case "jf_tebukuro":
		return 40
	case "jm_kumo":
		return 41
	case "pf_dora":
		return 42
	case "pm_alex":
		return 43
	case "pm_santa":
		return 44
	case "zf_xiaobei":
		return 45
	case "zf_xiaoni":
		return 46
	case "zf_xiaoxiao":
		return 47
	case "zf_xiaoyi":
		return 48
	case "zm_yunjian":
		return 49
	case "zm_yunxi":
		return 50
	case "zm_yunxia":
		return 51
	case "zm_yunyang":
		return 52
	default:
		return -1
	}
}
