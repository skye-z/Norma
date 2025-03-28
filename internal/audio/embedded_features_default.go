//go:build !whisper_embedded && !sherpa_embedded

package audio

func EmbeddedWhisperEnabled() bool { return false }

func EmbeddedSherpaEnabled() bool { return false }
