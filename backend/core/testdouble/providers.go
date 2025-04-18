// Package testdouble 测试替身
package testdouble

import (
	"context"
	"sync"

	"norma/backend/core/audio"
	"norma/backend/core/llm"
	"norma/backend/core/memory"
)

// FakeRetriever 模拟检索器
type FakeRetriever struct {
	Chunks []memory.Chunk
	Err    error
}

// Search 模拟搜索
func (f FakeRetriever) Search(ctx context.Context, query string, topK int) ([]memory.Chunk, error) {
	_ = ctx
	_ = query
	_ = topK
	return f.Chunks, f.Err
}

// FakeLLM 模拟 LLM
type FakeLLM struct {
	Tokens []string
	Err    error
}

// Stream 模拟流式输出
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

// FakeTTS 模拟 TTS
type FakeTTS struct {
	mu       sync.Mutex
	Chunks   [][]byte
	StopHits int
}

// Stream 模拟流式合成
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

// Stop 模拟停止
func (f *FakeTTS) Stop(sessionID string) error {
	_ = sessionID
	f.mu.Lock()
	defer f.mu.Unlock()
	f.StopHits++
	return nil
}

// FakeASR 模拟 ASR
type FakeASR struct {
	Partials []string
	Final    string
}

// Stream 模拟流式识别
func (f FakeASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	_ = ctx
	for _, partial := range f.Partials {
		onPartial(partial)
	}
	onFinal(f.Final)
	return nil
}

var _ audio.ASR = FakeASR{}
