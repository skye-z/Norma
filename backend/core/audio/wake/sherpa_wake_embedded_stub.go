//go:build !windows || !cgo || !sherpa_embedded

package audio

import (
	"norma/backend/core/audio"
)

func NewSherpaEmbeddedWakeWord(encoderPath, decoderPath, joinerPath, tokensPath, keywordsFilePath, keyword string) audio.WakeWord {
	_ = encoderPath
	_ = decoderPath
	_ = joinerPath
	_ = tokensPath
	_ = keywordsFilePath
	_ = keyword
	return &audio.UnavailableWakeWord{Reason: "embedded sherpa wake is only supported on windows"}
}
