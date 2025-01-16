package index

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Extractor extracts plain text from files.
type Extractor interface {
	Extract(path string) (string, error)
}

// TextExtractor extracts from text-like files.
type TextExtractor struct{}

func (e TextExtractor) Extract(path string) (string, error) {
	if shouldSkipPath(path) {
		return "", fmt.Errorf("path ignored by filter")
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if likelyBinary(data) {
			return "", fmt.Errorf("binary content detected")
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

func likelyBinary(data []byte) bool {
	max := len(data)
	if max > 1024 {
		max = 1024
	}
	for i := 0; i < max; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
