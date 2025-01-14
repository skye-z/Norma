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
	onPartial func(string),
	onFinal func(string),
) error {
	if p.VAD == nil || p.ASR == nil {
		return errors.New("pipeline requires VAD and ASR")
	}

	detector := NewEndpointDetector(4)
	utterance := make([][]float32, 0, 16)

	for {
		frame, err := source.Next(ctx)
		if err != nil {
			if errors.Is(err, ErrEndOfStream) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		isSpeech, err := p.VAD.Detect(frame)
		if err != nil {
			return err
		}

		if isSpeech {
			copied := make([]float32, len(frame))
			copy(copied, frame)
			utterance = append(utterance, copied)
		}

		start, end := detector.Update(isSpeech)
		if start && onSpeechStart != nil {
			onSpeechStart()
		}

		if end {
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
		}
	}
}
