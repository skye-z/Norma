// Package audio 音频处理
package audio

import "strings"

// splitSentences 分割句子
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	seps := map[rune]struct{}{
		'\u3002': {}, '\uFF0C': {}, '\uFF1B': {}, ';': {}, '\uFF01': {}, '.': {}, '!': {}, '?': {}, '\uFF1F': {}, '\n': {},
	}

	parts := make([]string, 0, 8)
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		if _, ok := seps[r]; ok {
			p := strings.TrimSpace(cur.String())
			if p != "" {
				parts = append(parts, p)
			}
			cur.Reset()
		}
	}

	tail := strings.TrimSpace(cur.String())
	if tail != "" {
		parts = append(parts, tail)
	}

	return parts
}

// prioritizeSentenceParts 优先短句
func prioritizeSentenceParts(parts []string) []string {
	if len(parts) <= 1 {
		return parts
	}
	short := make([]string, 0, len(parts))
	long := make([]string, 0, len(parts))
	for _, part := range parts {
		if len([]rune(strings.TrimSpace(part))) <= 10 {
			short = append(short, part)
			continue
		}
		long = append(long, part)
	}
	return append(short, long...)
}
