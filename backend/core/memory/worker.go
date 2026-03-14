// Package memory 记忆服务
package memory

import (
	"context"
	"log"
	"strings"
)

// 索引任务消费者
type Worker struct {
	queue     Queue
	extractor Extractor
	sink      Sink
}

// 创建 Worker
func NewWorker(queue Queue, extractor Extractor) *Worker {
	return &Worker{queue: queue, extractor: extractor}
}

// 创建带 Sink 的 Worker
func NewWorkerWithSink(queue Queue, extractor Extractor, sink Sink) *Worker {
	return &Worker{queue: queue, extractor: extractor, sink: sink}
}

// 运行 Worker
func (w *Worker) Run(ctx context.Context) error {
	for {
		task, err := w.queue.Dequeue(ctx)
		if err != nil {
			return nil
		}

		switch task.Kind {
		case "delete":
			if w.sink != nil {
				if err := w.sink.Delete(task.Path); err != nil {
					log.Printf("[index] delete failed path=%s err=%v", task.Path, err)
				}
			}
			log.Printf("[index] delete path=%s", task.Path)
		default:
			text, err := w.extractor.Extract(task.Path)
			if err != nil {
				log.Printf("[index] extract failed path=%s err=%v", task.Path, err)
				continue
			}
			chunks := chunkStructured(text, 500, 80)
			if w.sink != nil {
				if err := w.sink.Upsert(task.Path, chunks); err != nil {
					log.Printf("[index] upsert failed path=%s err=%v", task.Path, err)
					continue
				}
			}
			log.Printf("[index] upsert path=%s chunks=%d", task.Path, len(chunks))
		}
	}
}

// chunkText 文本分块
func chunkText(text string, maxChars int) []string {
	return chunkStructured(text, maxChars, 0)
}

// chunkStructured 结构化文本分块
func chunkStructured(text string, maxChars int, overlapChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 500
	}
	if overlapChars < 0 {
		overlapChars = 0
	}
	if overlapChars >= maxChars {
		overlapChars = maxChars / 4
	}

	segments := splitStructuredSegments(text)
	if len(segments) == 0 {
		segments = []string{text}
	}

	out := make([]string, 0, len(segments))
	current := ""
	appendChunk := func(chunk string) {
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			out = append(out, chunk)
		}
	}

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		if len([]rune(seg)) > maxChars {
			if current != "" {
				appendChunk(current)
				current = ""
			}
			for _, c := range splitByRunes(seg, maxChars, overlapChars) {
				appendChunk(c)
			}
			continue
		}

		candidate := seg
		if current != "" {
			candidate = current + "\n\n" + seg
		}

		if len([]rune(candidate)) <= maxChars {
			current = candidate
			continue
		}

		if current != "" {
			appendChunk(current)
			if overlapChars > 0 {
				tail := lastRunes(current, overlapChars)
				if tail != "" {
					current = strings.TrimSpace(tail + "\n\n" + seg)
					if len([]rune(current)) > maxChars {
						for _, c := range splitByRunes(current, maxChars, overlapChars) {
							appendChunk(c)
						}
						current = ""
					}
					continue
				}
			}
		}
		current = seg
	}

	if current != "" {
		appendChunk(current)
	}
	return out
}

// splitStructuredSegments 分割结构化段落
func splitStructuredSegments(text string) []string {
	lines := strings.Split(text, "\n")
	segments := make([]string, 0, len(lines)/2)
	var buf strings.Builder
	inCode := false

	flush := func() {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			segments = append(segments, s)
		}
		buf.Reset()
	}

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			buf.WriteString(line)
			if inCode {
				flush()
			}
			inCode = !inCode
			continue
		}

		if inCode {
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			buf.WriteString(line)
			continue
		}

		if strings.HasPrefix(trim, "#") {
			flush()
			buf.WriteString(trim)
			flush()
			continue
		}

		if trim == "" {
			flush()
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(line)
	}

	flush()
	return segments
}

// splitByRunes 按字符分块
func splitByRunes(text string, maxChars int, overlapChars int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 500
	}
	if overlapChars < 0 {
		overlapChars = 0
	}
	if overlapChars >= maxChars {
		overlapChars = maxChars / 4
	}

	out := make([]string, 0, len(runes)/maxChars+1)
	step := maxChars - overlapChars
	if step <= 0 {
		step = maxChars
	}
	for i := 0; i < len(runes); i += step {
		end := i + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, strings.TrimSpace(string(runes[i:end])))
		if end == len(runes) {
			break
		}
	}
	return out
}

// lastRunes 获取最后 n 个字符
func lastRunes(text string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(text))
	if len(r) <= n {
		return string(r)
	}
	return string(r[len(r)-n:])
}
