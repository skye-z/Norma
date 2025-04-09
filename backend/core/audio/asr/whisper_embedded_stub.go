//go:build !cgo || !whisper_embedded

package audio

import (
	"fmt"

	"norma/backend/core/audio"
)

func NewWhisperEmbeddedASR(modelPath, language string, threads int) (audio.ASR, error) {
	_ = modelPath
	_ = language
	_ = threads
	return nil, fmt.Errorf("embedded whisper requires build tags: cgo, whisper_embedded")
}
