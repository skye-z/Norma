package memory

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
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
	case ".docx":
		return extractDOCX(path)
	case ".pdf":
		return extractPDF(path)
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

func extractDOCX(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if strings.EqualFold(f.Name, "word/document.xml") {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			b, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			text := stripXML(string(b))
			text = cleanWhitespace(text)
			if text == "" {
				return "", fmt.Errorf("empty docx content")
			}
			return text, nil
		}
	}

	return "", fmt.Errorf("word/document.xml not found")
}

func extractPDF(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	text := extractPDFLiteralStrings(b)
	text = cleanWhitespace(text)
	if text == "" {
		return "", fmt.Errorf("no extractable pdf text")
	}
	return text, nil
}

func extractPDFLiteralStrings(data []byte) string {
	var out bytes.Buffer
	in := false
	escape := false
	depth := 0

	for _, c := range data {
		if !in {
			if c == '(' {
				in = true
				depth = 1
				escape = false
			}
			continue
		}

		if escape {
			escape = false
			if c == 'n' || c == 'r' || c == 't' {
				out.WriteByte(' ')
			} else {
				out.WriteByte(c)
			}
			continue
		}

		if c == '\\' {
			escape = true
			continue
		}
		if c == '(' {
			depth++
			continue
		}
		if c == ')' {
			depth--
			if depth <= 0 {
				in = false
				out.WriteByte(' ')
			}
			continue
		}

		if unicode.IsPrint(rune(c)) {
			out.WriteByte(c)
		}
	}

	return out.String()
}

var xmlTagRE = regexp.MustCompile(`<[^>]+>`)

func stripXML(s string) string {
	s = strings.ReplaceAll(s, "</w:p>", "\n")
	s = strings.ReplaceAll(s, "<w:br/>", "\n")
	s = xmlTagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return s
}

func cleanWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.TrimSpace(strings.Join(fields, " "))
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
