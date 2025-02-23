package testdouble

import (
	"context"
	"sync"

	"norma/internal/audio"
	"norma/internal/index"
	"norma/internal/llm"
)

type FakeRetriever struct {
	Chunks []index.Chunk
	Err    error
}

func (f FakeRetriever) Search(ctx context.Context, query string, topK int) ([]index.Chunk, error) {
	_ = ctx
	_ = query
	_ = topK
	return f.Chunks, f.Err
}

type FakeLLM struct {
	Tokens []string
	Err    error
}

func (f FakeLLM) Stream(ctx context.Context, req llm.Request, onToken func(string)) error {
	_ = req
	if f.Err != nil {
		return f.Err
	}
	for _, token := range f.Tokens {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			onToken(token)
		}
	}
	return nil
}

type FakeTTS struct {
	mu       sync.Mutex
	Chunks   [][]byte
	StopHits int
}

func (f *FakeTTS) Stream(ctx context.Context, text string, onChunk func([]byte)) error {
	_ = text
	for _, chunk := range f.Chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			onChunk(chunk)
		}
	}
	return nil
}
func (f *FakeTTS) Stop(sessionID string) error {
	_ = sessionID
	f.mu.Lock()
	defer f.mu.Unlock()
	f.StopHits++
	return nil
}

type FakeASR struct {
	Partials []string
	Final    string
}

func (f FakeASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	_ = ctx
	for _, partial := range f.Partials {
		onPartial(partial)
	}
	onFinal(f.Final)
	return nil
}

var _ audio.ASR = FakeASR{}
