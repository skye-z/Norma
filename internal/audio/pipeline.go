package audio

import (
	"context"
	"errors"
)

// Run consumes frames and triggers ASR final on endpoint.
func (p *Pipeline) Run(
	ctx context.Context,
	source FrameSource,
	onSpeechStart func(),
	onSpeechEnd func(),
	onPartial func(string),
	onFinal func(string),
) error {
	if p.VAD == nil || p.ASR == nil {
		return errors.New("pipeline requires VAD and ASR")
	}

	silenceFrames := p.SilenceFrames
	if silenceFrames <= 0 {
		silenceFrames = 4
	}
	detector := NewEndpointDetector(silenceFrames)
	utterance := make([][]float32, 0, 16)
	wakeMode := p.WakeMode
	if wakeMode == "" {
		wakeMode = WakeModeVADOnly
	}
	armed := wakeMode == WakeModeVADOnly || p.WakeWord == nil

	for {
		frame, err := source.Next(ctx)
		if err != nil {
			if errors.Is(err, ErrEndOfStream) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		processed := frame
		for _, pre := range p.Preprocessors {
			if pre == nil {
				continue
			}
			processed = pre.Process(processed)
		}

		if !armed && p.WakeWord != nil {
			pcm := floatToPCM16(processed)
			matched, err := p.WakeWord.Detect(pcm)
			if err != nil {
				return err
			}
			if matched {
				armed = true
			}
			continue
		}

		isSpeech, err := p.VAD.Detect(processed)
		if err != nil {
			return err
		}

		if isSpeech {
			copied := make([]float32, len(processed))
			copy(copied, processed)
			utterance = append(utterance, copied)
		}

		start, end := detector.Update(isSpeech)
		if start && onSpeechStart != nil {
			onSpeechStart()
		}

		if end {
			if onSpeechEnd != nil {
				onSpeechEnd()
			}
			if fasr, ok := p.ASR.(FrameASR); ok {
				if err := fasr.TranscribeFrames(ctx, utterance, onPartial, onFinal); err != nil {
					return err
				}
			} else {
				if err := p.ASR.Stream(ctx, onPartial, onFinal); err != nil {
					return err
				}
			}
			utterance = utterance[:0]
			if wakeMode == WakeModeWakeWordVAD && p.WakeWord != nil {
				armed = false
			}
		}
	}
}

func floatToPCM16(frame []float32) []int16 {
	out := make([]int16, len(frame))
	for i, sample := range frame {
		if sample > 1 {
			sample = 1
		}
		if sample < -1 {
			sample = -1
		}
		out[i] = int16(sample * 32767)
	}
	return out
}
