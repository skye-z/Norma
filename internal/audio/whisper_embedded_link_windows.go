//go:build windows && cgo && whisper_embedded

package audio

/*
#cgo windows LDFLAGS: -fopenmp
*/
import "C"
