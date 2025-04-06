package audio

import (
	"context"
	"errors"
	"log"
)

const minPostWakeHoldFrames = 75

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
	wakeBufferFrames := p.WakeBufferFrames
	if wakeBufferFrames <= 0 {
		wakeBufferFrames = 1
	}
	wakeBuffer := make([][]float32, 0, wakeBufferFrames)
	wakeMode := p.WakeMode
	if wakeMode == "" {
		wakeMode = WakeModeVADOnly
	}
	armed := wakeMode == WakeModeVADOnly || p.WakeWord == nil
	postWakeHoldFrames := 0
	log.Printf("[pipeline] start wake_mode=%s armed=%v allow_barge_in=%v", wakeMode, armed, p.AllowBargeIn)

	flushUtterance := func(reason string) error {
		if len(utterance) == 0 {
			return nil
		}
		log.Printf("[pipeline] flush utterance reason=%s utterance_frames=%d", reason, len(utterance))
		if onSpeechEnd != nil {
			onSpeechEnd()
		}
		if fasr, ok := p.ASR.(FrameASR); ok {
			log.Printf("[pipeline] transcribe via FrameASR frames=%d", len(utterance))
			if err := fasr.TranscribeFrames(ctx, utterance, onPartial, onFinal); err != nil {
				log.Printf("[pipeline] frame asr error: %v", err)
				return err
			}
		} else {
			log.Printf("[pipeline] transcribe via streaming ASR")
			if err := p.ASR.Stream(ctx, onPartial, onFinal); err != nil {
				log.Printf("[pipeline] asr stream error: %v", err)
				return err
			}
		}
		utterance = utterance[:0]
		if wakeMode == WakeModeWakeWordVAD && p.WakeWord != nil {
			armed = false
			postWakeHoldFrames = 0
			wakeBuffer = wakeBuffer[:0]
			detector = NewEndpointDetector(silenceFrames)
		}
		return nil
	}

	for {
		frame, err := source.Next(ctx)
		if err != nil {
			if errors.Is(err, ErrEndOfStream) {
				if flushErr := flushUtterance("end_of_stream"); flushErr != nil {
					return flushErr
				}
				return nil
			}
			if errors.Is(err, context.Canceled) {
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
			buffered := cloneFrame(processed)
			wakeBuffer = append(wakeBuffer, buffered)
			if len(wakeBuffer) > wakeBufferFrames {
				wakeBuffer = append([][]float32(nil), wakeBuffer[len(wakeBuffer)-wakeBufferFrames:]...)
			}
			pcm := floatToPCM16(processed)
			matched, err := p.WakeWord.Detect(pcm)
			if err != nil {
				log.Printf("[pipeline] wake detect error: %v", err)
				return err
			}
			if matched {
				log.Printf("[pipeline] wake detected buffered_frames=%d", len(wakeBuffer))
				armed = true
				postWakeHoldFrames = wakeBufferFrames
				if postWakeHoldFrames < minPostWakeHoldFrames {
					postWakeHoldFrames = minPostWakeHoldFrames
				}
				utterance = append(utterance[:0], cloneFrames(wakeBuffer)...)
				wakeBuffer = wakeBuffer[:0]
				if onSpeechStart != nil {
					log.Printf("[pipeline] speech start from wake utterance_frames=%d", len(utterance))
					onSpeechStart()
				}
				_, _ = detector.Update(true)
			}
			continue
		}

		forceSpeech := false
		if postWakeHoldFrames > 0 {
			forceSpeech = true
			postWakeHoldFrames--
		}

		isSpeech, err := p.VAD.Detect(processed)
		if err != nil {
			log.Printf("[pipeline] vad detect error: %v", err)
			return err
		}

		if isSpeech || forceSpeech {
			utterance = append(utterance, cloneFrame(processed))
		}

		start, end := detector.Update(isSpeech || forceSpeech)
		if p.AllowBargeIn && isSpeech && onSpeechStart != nil {
			log.Printf("[pipeline] barge-in speech detected")
			onSpeechStart()
		}
		if start && onSpeechStart != nil {
			log.Printf("[pipeline] speech start utterance_frames=%d", len(utterance))
			onSpeechStart()
		}

		if end {
			log.Printf("[pipeline] speech end utterance_frames=%d", len(utterance))
			if err := flushUtterance("endpoint"); err != nil {
				return err
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

func cloneFrame(frame []float32) []float32 {
	copied := make([]float32, len(frame))
	copy(copied, frame)
	return copied
}

func cloneFrames(frames [][]float32) [][]float32 {
	out := make([][]float32, 0, len(frames))
	for _, frame := range frames {
		out = append(out, cloneFrame(frame))
	}
	return out
}
