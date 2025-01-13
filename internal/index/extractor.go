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
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}
