// Package audio 音频处理
package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Live2D 控制标识不朗读
var live2dMarkRe = regexp.MustCompile(`\{\{[^}]*\}\}?`)

// 调用 macOS 系统 say 命令合成语音，每个分句合成一个 PCM 块
type SystemSayTTS struct {
	Voice string
	Rate  int // 每分钟字数，0 使用系统默认

	mu      sync.Mutex
	cancels map[int]context.CancelFunc
	seq     int
}

func NewSystemSayTTS(voice string, rate int) *SystemSayTTS {
	if strings.TrimSpace(voice) == "" {
		voice = "Tingting"
	}
	return &SystemSayTTS{Voice: voice, Rate: rate, cancels: map[int]context.CancelFunc{}}
}

func (s *SystemSayTTS) SampleRate() int { return 24000 }

func (s *SystemSayTTS) Channels() int { return 1 }

func (s *SystemSayTTS) Encoding() string { return "pcm_s16le" }

func (s *SystemSayTTS) SupportsProsodyHints() bool { return false }

func (s *SystemSayTTS) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	text = cleanSpeechText(text)
	if text == "" {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.seq++
	id := s.seq
	s.cancels[id] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.cancels, id)
		s.mu.Unlock()
	}()

	dir, err := os.MkdirTemp("", "norma-say-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.wav")
	args := []string{"-v", s.Voice, "--file-format=WAVE", "--data-format=LEI16@24000", "-o", out}
	if s.Rate > 0 {
		args = append(args, "-r", fmt.Sprint(s.Rate))
	}
	args = append(args, "--", text)
	if b, err := exec.CommandContext(ctx, "say", args...).CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("say: %w: %s", err, strings.TrimSpace(string(b)))
	}
	pcm, err := readWAVData(out)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if onChunk != nil && len(pcm) > 0 {
		onChunk(pcm)
	}
	return nil
}

func (s *SystemSayTTS) Stop(sessionID string) error {
	_ = sessionID
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cancels {
		c()
	}
	return nil
}

// 去掉 Markdown 标记等不适合朗读的符号
func cleanSpeechText(t string) string {
	t = live2dMarkRe.ReplaceAllString(t, "")
	r := strings.NewReplacer("*", "", "#", "", "`", "", "_", " ", ">", " ", "~", "", "\n", "，")
	return strings.TrimSpace(r.Replace(t))
}

// 读取 WAV 文件中 data 块的原始 PCM
func readWAVData(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 12 || string(b[0:4]) != "RIFF" {
		return nil, fmt.Errorf("not a wav file")
	}
	for i := 12; i+8 <= len(b); {
		size := int(binary.LittleEndian.Uint32(b[i+4 : i+8]))
		if string(b[i:i+4]) == "data" {
			end := i + 8 + size
			if end > len(b) || size == 0 {
				end = len(b)
			}
			return b[i+8 : end], nil
		}
		i += 8 + size + size%2
	}
	return nil, fmt.Errorf("wav data chunk not found")
}
