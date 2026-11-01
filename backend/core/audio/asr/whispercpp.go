package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"norma/backend/core/audio"
)

var timeTagRe = regexp.MustCompile(`\[[^\]]+\]`)

// transcribes frames by invoking whisper.cpp CLI.
type WhisperCPPASR struct {
	BinaryPath string
	ModelPath  string
	Language   string
	Threads    int
	SampleRate int
	TempDir    string
	// 起始提示，用于让 Whisper 倾向输出简体中文；为空时使用默认提示
	Prompt string
}

const defaultWhisperPrompt = "以下是普通话的句子，使用简体中文。"

func NewWhisperCPPASR(binaryPath, modelPath, language string, threads int) *WhisperCPPASR {
	if strings.TrimSpace(language) == "" {
		language = "zh"
	}
	if threads <= 0 {
		threads = 4
	}
	return &WhisperCPPASR{
		BinaryPath: strings.TrimSpace(binaryPath),
		ModelPath:  strings.TrimSpace(modelPath),
		Language:   language,
		Threads:    threads,
		SampleRate: 16000,
	}
}

func (w *WhisperCPPASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	// Stream without frames is unsupported for whisper.cpp adapter.
	_ = ctx
	_ = onPartial
	_ = onFinal
	return fmt.Errorf("whispercpp requires frame input, use TranscribeFrames")
}

func (w *WhisperCPPASR) TranscribeFrames(
	ctx context.Context,
	frames [][]float32,
	onPartial func(string),
	onFinal func(string),
) error {
	if w.BinaryPath == "" || w.ModelPath == "" {
		return fmt.Errorf("whispercpp binary/model path is empty")
	}

	wavPath, cleanup, err := w.writeTempWav(frames)
	if err != nil {
		return err
	}
	defer cleanup()

	if onPartial != nil {
		onPartial("正在识别...")
	}

	prompt := strings.TrimSpace(w.Prompt)
	if prompt == "" {
		prompt = defaultWhisperPrompt
	}
	text, err := w.runOnce(ctx, wavPath, prompt)
	if err != nil {
		return err
	}
	// 音频很短或不清晰时，Whisper 会把起始提示词原样吐出来而丢掉真实内容。
	// 或被判为幻觉（如“请不吝点赞”）时，先不带提示词重识别一次，内容就不会丢；重识别后的繁体字由后处理统一为简体
	if text == "" || PostProcessASRText(text, "") == "" {
		if retry, rerr := w.runOnce(ctx, wavPath, ""); rerr == nil && PostProcessASRText(retry, "") != "" {
			log.Printf("[asr] prompt echo %q, retried without prompt: %q", text, retry)
			text = retry
		}
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("whispercpp: %w", audio.ErrNoSpeech)
	}

	if onFinal != nil {
		onFinal(text)
	}
	return nil
}

// runOnce 调用一次 whisper.cpp；prompt 为空时不带起始提示
func (w *WhisperCPPASR) runOnce(ctx context.Context, wavPath, prompt string) (string, error) {
	args := []string{
		"-m", w.ModelPath,
		"-f", wavPath,
		"-l", w.Language,
		"-t", fmt.Sprintf("%d", w.Threads),
		"-nt", "-np",
	}
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}
	// 识别文本在标准输出；模型加载等日志写在标准错误，不能混入文本
	cmd := exec.CommandContext(ctx, w.BinaryPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run whispercpp: %w, stderr=%s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(normalizeWhisperOutput(string(out))), nil
}

func (w *WhisperCPPASR) writeTempWav(frames [][]float32) (string, func(), error) {
	dir := w.TempDir
	if strings.TrimSpace(dir) == "" {
		var err error
		dir, err = os.MkdirTemp("", "norma-whisper-")
		if err != nil {
			return "", nil, err
		}
	} else {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", nil, err
		}
	}

	path := filepath.Join(dir, "input.wav")
	f, err := os.Create(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	pcm := flattenToPCM16(frames)
	dataLen := uint32(len(pcm) * 2)

	if err := writeWavHeader(f, uint32(w.SampleRate), dataLen); err != nil {
		return "", nil, err
	}
	if err := binary.Write(f, binary.LittleEndian, pcm); err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.Remove(path)
		if strings.Contains(filepath.Base(dir), "norma-whisper-") {
			_ = os.Remove(dir)
		}
	}
	return path, cleanup, nil
}

func flattenToPCM16(frames [][]float32) []int16 {
	out := make([]int16, 0, 160*len(frames))
	for _, frame := range frames {
		for _, x := range frame {
			v := int(math.Round(float64(x) * 32767.0))
			if v > 32767 {
				v = 32767
			} else if v < -32768 {
				v = -32768
			}
			out = append(out, int16(v))
		}
	}
	return out
}

func writeWavHeader(f *os.File, sampleRate uint32, dataLen uint32) error {
	byteRate := sampleRate * 2
	blockAlign := uint16(2)
	bitsPerSample := uint16(16)
	riffSize := 36 + dataLen

	header := []any{
		[]byte("RIFF"), riffSize, []byte("WAVE"),
		[]byte("fmt "), uint32(16), uint16(1), uint16(1), sampleRate, byteRate, blockAlign, bitsPerSample,
		[]byte("data"), dataLen,
	}

	for _, h := range header {
		if err := binary.Write(f, binary.LittleEndian, h); err != nil {
			return err
		}
	}
	return nil
}

func normalizeWhisperOutput(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		l = timeTagRe.ReplaceAllString(l, "")
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, " ")
}
