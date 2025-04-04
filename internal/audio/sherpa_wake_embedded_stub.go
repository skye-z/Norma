//go:build !windows || !cgo || !sherpa_embedded

package audio

func NewSherpaEmbeddedWakeWord(encoderPath, decoderPath, joinerPath, tokensPath, keywordsFilePath, keyword string) WakeWord {
	_ = encoderPath
	_ = decoderPath
	_ = joinerPath
	_ = tokensPath
	_ = keywordsFilePath
	_ = keyword
	return &UnavailableWakeWord{Reason: "embedded sherpa wake is only supported on windows"}
}
