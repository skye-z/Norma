package audio

import "context"

// VAD detects speech activity from audio frames.
type VAD interface {
	Detect(frame []float32) (bool, error)
}

// WakeWord detects wake phrase from PCM stream.
type WakeWord interface {
	Detect(frame []int16) (bool, error)
}

// ASR streams transcription events.
type ASR interface {
	Stream(ctx context.Context, onPartial func(text string), onFinal func(text string)) error
}

// FrameASR can transcribe utterance frames directly.
type FrameASR interface {
	ASR
	TranscribeFrames(ctx context.Context, frames [][]float32, onPartial func(text string), onFinal func(text string)) error
}

// TTS streams synthesized audio chunks and supports interruption.
type TTS interface {
	Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error
	Stop(sessionID string) error
}

// Pipeline coordinates voice-related components.
type Pipeline struct {
	VAD      VAD
	WakeWord WakeWord
	ASR      ASR
	TTS      TTS
}
