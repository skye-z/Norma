// Package audio 音频处理
package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SherpaWakeWord Sherpa 唤醒词检测器
type SherpaWakeWord struct {
	BinaryPath       string
	EncoderPath      string
	DecoderPath      string
	JoinerPath       string
	TokensPath       string
	KeywordsFilePath string
	Keyword          string
	SampleRate       int
	WindowFrames     int
	DetectInterval   int
	Threshold        float32
	TempDir          string
	commandRunner    func(args []string) (string, error)

	mu             sync.Mutex
	frames         [][]int16
	detectTick     int
	lastDetectedAt time.Time
}

// sherpaKWSEvent 唤醒词事件
type sherpaKWSEvent struct {
	Keyword string `json:"keyword"`
}

// NewSherpaWakeWord 创建 Sherpa 唤醒词检测器
func NewSherpaWakeWord(binaryPath, encoderPath, decoderPath, joinerPath, tokensPath, keywordsFilePath, keyword string) *SherpaWakeWord {
	return &SherpaWakeWord{
		BinaryPath:       strings.TrimSpace(binaryPath),
		EncoderPath:      strings.TrimSpace(encoderPath),
		DecoderPath:      strings.TrimSpace(decoderPath),
		JoinerPath:       strings.TrimSpace(joinerPath),
		TokensPath:       strings.TrimSpace(tokensPath),
		KeywordsFilePath: strings.TrimSpace(keywordsFilePath),
		Keyword:          strings.TrimSpace(keyword),
		SampleRate:       16000,
		WindowFrames:     50,
		DetectInterval:   10,
		Threshold:        0.01,
		commandRunner: func(args []string) (string, error) {
			cmd := exec.Command(binaryPath, args...)
			out, err := cmd.CombinedOutput()
			return string(out), err
		},
	}
}

func (s *SherpaWakeWord) Detect(frame []int16) (bool, error) {
	if s.BinaryPath == "" || s.EncoderPath == "" || s.DecoderPath == "" || s.JoinerPath == "" || s.TokensPath == "" || s.KeywordsFilePath == "" {
		return false, fmt.Errorf("sherpa wake assets incomplete")
	}
	if !hasSignal(frame, s.Threshold) {
		return false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	copied := append([]int16(nil), frame...)
	s.frames = append(s.frames, copied)
	if s.WindowFrames <= 0 {
		s.WindowFrames = 50
	}
	if len(s.frames) > s.WindowFrames {
		s.frames = append([][]int16(nil), s.frames[len(s.frames)-s.WindowFrames:]...)
	}

	s.detectTick++
	if s.DetectInterval <= 0 {
		s.DetectInterval = 10
	}
	if len(s.frames) < s.WindowFrames/2 || s.detectTick%s.DetectInterval != 0 {
		return false, nil
	}
	if !s.lastDetectedAt.IsZero() && time.Since(s.lastDetectedAt) < time.Second {
		return false, nil
	}

	wavPath, cleanup, err := s.writeWindowWav(s.frames)
	if err != nil {
		return false, err
	}
	defer cleanup()

	args := []string{
		"--encoder", s.EncoderPath,
		"--decoder", s.DecoderPath,
		"--joiner", s.JoinerPath,
		"--tokens", s.TokensPath,
		"--keywords-file", s.KeywordsFilePath,
		wavPath,
	}
	out, err := s.commandRunner(args)
	if err != nil {
		return false, fmt.Errorf("run sherpa wake: %w, output=%s", err, strings.TrimSpace(out))
	}
	matched := sherpaWakeMatched(out, s.Keyword)
	if matched {
		s.lastDetectedAt = time.Now()
	}
	return matched, nil
}

func (s *SherpaWakeWord) writeWindowWav(frames [][]int16) (string, func(), error) {
	dir := s.TempDir
	if strings.TrimSpace(dir) == "" {
		var err error
		dir, err = os.MkdirTemp("", "norma-sherpa-wake-")
		if err != nil {
			return "", nil, err
		}
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "wake.wav")
	f, err := os.Create(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	pcm := flattenPCM(frames)
	dataLen := uint32(len(pcm) * 2)
	if err := writeWavHeader(f, uint32(s.SampleRate), dataLen); err != nil {
		return "", nil, err
	}
	if err := writePCM16(f, pcm); err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.Remove(path)
		if strings.Contains(filepath.Base(dir), "norma-sherpa-wake-") {
			_ = os.Remove(dir)
		}
	}
	return path, cleanup, nil
}

func flattenPCM(frames [][]int16) []int16 {
	out := make([]int16, 0, len(frames)*160)
	for _, frame := range frames {
		out = append(out, frame...)
	}
	return out
}

func writePCM16(f *os.File, pcm []int16) error {
	for _, sample := range pcm {
		if err := binaryWriteLittleEndianInt16(f, sample); err != nil {
			return err
		}
	}
	return nil
}

func binaryWriteLittleEndianInt16(f *os.File, v int16) error {
	buf := []byte{byte(v), byte(v >> 8)}
	_, err := f.Write(buf)
	return err
}

func sherpaWakeMatched(out string, expected string) bool {
	expected = strings.TrimSpace(strings.ToLower(expected))
	if expected != "" && strings.Contains(strings.ToLower(out), expected) {
		return true
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt sherpaKWSEvent
		if err := json.Unmarshal([]byte(line), &evt); err == nil {
			if expected == "" && strings.TrimSpace(evt.Keyword) != "" {
				return true
			}
			if strings.EqualFold(strings.TrimSpace(evt.Keyword), expected) {
				return true
			}
		}
	}
	return false
}

func hasSignal(frame []int16, threshold float32) bool {
	if threshold <= 0 {
		threshold = 0.01
	}
	limit := int16(threshold * 32767)
	if limit < 1 {
		limit = 1
	}
	for _, sample := range frame {
		if sample > limit || sample < -limit {
			return true
		}
	}
	return false
}
