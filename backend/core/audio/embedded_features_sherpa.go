//go:build (windows || darwin) && cgo && sherpa_embedded && !whisper_embedded

package audio

func EmbeddedWhisperEnabled() bool { return false }

func EmbeddedSherpaEnabled() bool { return true }
