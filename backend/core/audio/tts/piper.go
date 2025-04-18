// Package audio 音频处理
package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// PiperTTS Piper TTS
type PiperTTS struct {
	BinaryPath string
	ModelPath  string
	Speaker    string

	mu     sync.Mutex
	cancel context.CancelFunc
}

// SampleRate 获取采样率
func (p *PiperTTS) SampleRate() int { return 22050 }

// Channels 获取声道数
func (p *PiperTTS) Channels() int { return 1 }

// Encoding 获取编码格式
func (p *PiperTTS) Encoding() string { return "pcm_s16le" }

// SupportsProsodyHints 是否支持韵律提示
func (p *PiperTTS) SupportsProsodyHints() bool { return false }

// NewPiperTTS 创建 Piper TTS
func NewPiperTTS(binaryPath, modelPath, speaker string) *PiperTTS {
	return &PiperTTS{
		BinaryPath: strings.TrimSpace(binaryPath),
		ModelPath:  strings.TrimSpace(modelPath),
		Speaker:    strings.TrimSpace(speaker),
	}
}

// Stream 流式合成
func (p *PiperTTS) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	if p.BinaryPath == "" || p.ModelPath == "" {
		return fmt.Errorf("piper binary/model path is empty")
	}

	parts := prioritizeSentenceParts(splitSentences(text))
	if len(parts) == 0 {
		parts = []string{text}
	}

	streamCtx, cancel := context.WithCancel(ctx)
	p.mu.Lock()
	p.cancel = cancel
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.cancel = nil
		p.mu.Unlock()
		cancel()
	}()

	for _, part := range parts {
		if err := p.streamOnce(streamCtx, strings.TrimSpace(part), onChunk); err != nil {
			return err
		}
	}
	return nil
}

func (p *PiperTTS) streamOnce(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	if text == "" {
		return nil
	}

	args := []string{"--model", p.ModelPath, "--output-raw"}
	if p.Speaker != "" {
		args = append(args, "--speaker", p.Speaker)
	}

	cmd := exec.CommandContext(ctx, p.BinaryPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		_, _ = io.WriteString(stdin, text+"\n")
		_ = stdin.Close()
	}()

	buf := make([]byte, 4096)
	for {
		n, readErr := stdout.Read(buf)
		if n > 0 && onChunk != nil {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			onChunk(chunk)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			_ = cmd.Wait()
			return readErr
		}
	}

	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("run piper: %w, stderr=%s", err, msg)
		}
		return fmt.Errorf("run piper: %w", err)
	}
	return nil
}

func (p *PiperTTS) Stop(sessionID string) error {
	_ = sessionID
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}
