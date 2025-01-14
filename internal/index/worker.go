package index

import (
	"context"
	"log"
	"strings"
)

// Worker consumes indexing tasks.
type Worker struct {
	queue     Queue
	extractor Extractor
	sink      Sink
}

func NewWorker(queue Queue, extractor Extractor) *Worker {
	return &Worker{queue: queue, extractor: extractor}
}

func NewWorkerWithSink(queue Queue, extractor Extractor, sink Sink) *Worker {
	return &Worker{queue: queue, extractor: extractor, sink: sink}
}

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
			chunks := chunkText(text, 500)
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

func chunkText(text string, maxChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 500
	}

	out := make([]string, 0, len(text)/maxChars+1)
	runes := []rune(text)
	for i := 0; i < len(runes); i += maxChars {
		end := i + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, strings.TrimSpace(string(runes[i:end])))
	}
	return out
}
