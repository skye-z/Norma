//go:build cgo && whisper_embedded && !sherpa_embedded

package audio

func EmbeddedWhisperEnabled() bool { return true }

func EmbeddedSherpaEnabled() bool { return false }
