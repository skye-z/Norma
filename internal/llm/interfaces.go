package llm

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Request defines LLM input.
type Request struct {
	SessionID string
	Prompt    string
	Context   []string
}

// Streamer streams generated tokens.
type Streamer interface {
	Stream(ctx context.Context, req Request, onToken func(string)) error
}

// MockStreamer is development-only LLM streamer.
type MockStreamer struct{}

func (m MockStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	_ = m
	text := "收到，我先给你结论：这是一个可运行的 mock 语音链路。"
	if strings.TrimSpace(req.Prompt) != "" {
		text += " 你刚刚说的是：" + strings.TrimSpace(req.Prompt) + "。"
	}
	if len(req.Context) > 0 {
		text += " 我也参考了本地检索结果。"
	}

	for _, token := range strings.Split(text, "") {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			onToken(token)
			time.Sleep(25 * time.Millisecond)
		}
	}

	return nil
}

type UnavailableStreamer struct {
	Reason string
}

func (u UnavailableStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	_ = ctx
	_ = req
	_ = onToken
	if strings.TrimSpace(u.Reason) == "" {
		return errors.New("llm unavailable")
	}
	return errors.New("llm unavailable: " + strings.TrimSpace(u.Reason))
}
