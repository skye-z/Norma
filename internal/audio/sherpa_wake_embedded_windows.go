//go:build windows && cgo && sherpa_embedded

package audio

import (
	"fmt"
	"strings"
	"sync"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-windows"
)

type SherpaEmbeddedWakeWord struct {
	EncoderPath      string
	DecoderPath      string
	JoinerPath       string
	TokensPath       string
	KeywordsFilePath string
	Keyword          string
	SampleRate       int
	DetectInterval   int

	mu             sync.Mutex
	spotter        *sherpa.KeywordSpotter
	stream         *sherpa.OnlineStream
	detectTick     int
	lastDetectedAt time.Time
}

func NewSherpaEmbeddedWakeWord(encoderPath, decoderPath, joinerPath, tokensPath, keywordsFilePath, keyword string) WakeWord {
	return &SherpaEmbeddedWakeWord{
		EncoderPath:      strings.TrimSpace(encoderPath),
		DecoderPath:      strings.TrimSpace(decoderPath),
		JoinerPath:       strings.TrimSpace(joinerPath),
		TokensPath:       strings.TrimSpace(tokensPath),
		KeywordsFilePath: strings.TrimSpace(keywordsFilePath),
		Keyword:          strings.TrimSpace(keyword),
		SampleRate:       16000,
		DetectInterval:   4,
	}
}

func (s *SherpaEmbeddedWakeWord) Detect(frame []int16) (bool, error) {
	if s.EncoderPath == "" || s.DecoderPath == "" || s.JoinerPath == "" || s.TokensPath == "" || s.KeywordsFilePath == "" {
		return false, fmt.Errorf("sherpa wake assets incomplete")
	}
	if len(frame) == 0 || !hasSignal(frame, 0.01) {
		return false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureRuntime(); err != nil {
		return false, err
	}

	s.detectTick++
	if s.DetectInterval <= 0 {
		s.DetectInterval = 4
	}

	s.stream.AcceptWaveform(s.SampleRate, pcmToFloat(frame))
	if s.detectTick%s.DetectInterval != 0 {
		return false, nil
	}

	for s.spotter.IsReady(s.stream) {
		s.spotter.Decode(s.stream)
	}
	result := s.spotter.GetResult(s.stream)
	if result == nil || strings.TrimSpace(result.Keyword) == "" {
		return false, nil
	}
	if !s.lastDetectedAt.IsZero() && time.Since(s.lastDetectedAt) < time.Second {
		return false, nil
	}
	if s.Keyword != "" && !strings.EqualFold(strings.TrimSpace(result.Keyword), s.Keyword) {
		return false, nil
	}
	s.lastDetectedAt = time.Now()
	s.spotter.Reset(s.stream)
	return true, nil
}

func (s *SherpaEmbeddedWakeWord) ensureRuntime() error {
	ensureSherpaDLLPath()
	if s.spotter != nil && s.stream != nil {
		return nil
	}
	config := &sherpa.KeywordSpotterConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: s.SampleRate,
			FeatureDim: 80,
		},
		ModelConfig: sherpa.OnlineModelConfig{
			Transducer: sherpa.OnlineTransducerModelConfig{
				Encoder: s.EncoderPath,
				Decoder: s.DecoderPath,
				Joiner:  s.JoinerPath,
			},
			Tokens:     s.TokensPath,
			NumThreads: 2,
			Provider:   "cpu",
		},
		MaxActivePaths:    4,
		KeywordsFile:      s.KeywordsFilePath,
		KeywordsScore:     1.0,
		KeywordsThreshold: 0.2,
	}
	spotter := sherpa.NewKeywordSpotter(config)
	if spotter == nil {
		return fmt.Errorf("create sherpa keyword spotter failed")
	}
	stream := sherpa.NewKeywordStream(spotter)
	if stream == nil {
		sherpa.DeleteKeywordSpotter(spotter)
		return fmt.Errorf("create sherpa keyword stream failed")
	}
	s.spotter = spotter
	s.stream = stream
	return nil
}

func pcmToFloat(frame []int16) []float32 {
	out := make([]float32, len(frame))
	for i, v := range frame {
		out[i] = float32(v) / 32768.0
	}
	return out
}
