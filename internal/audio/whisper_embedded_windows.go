//go:build cgo && whisper_embedded

package audio

import (
	"context"
	"fmt"
	"log"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type WhisperEmbeddedASR struct {
	ModelPath string
	Language  string
	Threads   int
}

func NewWhisperEmbeddedASR(modelPath, language string, threads int) (ASR, error) {
	if strings.TrimSpace(modelPath) == "" {
		return nil, fmt.Errorf("embedded whisper model path is empty")
	}
	if strings.TrimSpace(language) == "" {
		language = "zh"
	}
	if threads <= 0 {
		threads = 4
	}
	return &WhisperEmbeddedASR{ModelPath: strings.TrimSpace(modelPath), Language: language, Threads: threads}, nil
}

func (w *WhisperEmbeddedASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	_ = ctx
	_ = onPartial
	_ = onFinal
	return fmt.Errorf("embedded whisper requires frame input, use TranscribeFrames")
}

func (w *WhisperEmbeddedASR) TranscribeFrames(ctx context.Context, frames [][]float32, onPartial func(string), onFinal func(string)) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if onPartial != nil {
		onPartial("正在识别...")
	}
	log.Printf("[asr] whisper transcribe start frames=%d samples=%d model=%s", len(frames), len(flattenFloatFrames(frames)), w.ModelPath)

	model, err := whisper.New(w.ModelPath)
	if err != nil {
		log.Printf("[asr] whisper load model failed: %v", err)
		return fmt.Errorf("load embedded whisper model: %w", err)
	}
	defer model.Close()

	whisperCtx, err := model.NewContext()
	if err != nil {
		log.Printf("[asr] whisper create context failed: %v", err)
		return fmt.Errorf("create embedded whisper context: %w", err)
	}
	if err := whisperCtx.SetLanguage(w.Language); err != nil {
		log.Printf("[asr] whisper set language failed: %v", err)
		return fmt.Errorf("set embedded whisper language: %w", err)
	}
	whisperCtx.SetTranslate(false)

	samples := flattenFloatFrames(frames)
	if len(samples) == 0 {
		log.Printf("[asr] whisper empty audio")
		return fmt.Errorf("embedded whisper empty audio")
	}
	log.Printf("[asr] whisper using raw samples samples=%d", len(samples))
	if err := whisperCtx.Process(samples, nil, nil, nil); err != nil {
		log.Printf("[asr] whisper process failed: %v", err)
		return fmt.Errorf("embedded whisper process failed: %w", err)
	}

	segments := make([]string, 0, 8)
	for {
		seg, segErr := whisperCtx.NextSegment()
		if segErr != nil {
			break
		}
		text := strings.TrimSpace(seg.Text)
		if text != "" {
			segments = append(segments, text)
		}
	}
	finalText := strings.TrimSpace(strings.Join(segments, " "))
	if finalText == "" {
		log.Printf("[asr] whisper final text empty")
		return fmt.Errorf("embedded whisper returned empty text")
	}
	log.Printf("[asr] whisper final text=%q", finalText)
	if onFinal != nil {
		onFinal(finalText)
	}
	return nil
}

func flattenFloatFrames(frames [][]float32) []float32 {
	total := 0
	for _, frame := range frames {
		total += len(frame)
	}
	out := make([]float32, 0, total)
	for _, frame := range frames {
		out = append(out, frame...)
	}
	return out
}
