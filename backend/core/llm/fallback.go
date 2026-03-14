// Package llm LLM 接口
package llm

import "context"

// 降级流式响应器
type FallbackStreamer struct {
	Primary  Streamer
	Fallback Streamer
}

// 流式调用，优先主用，失败时降级
func (f FallbackStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	if f.Primary == nil {
		return f.Fallback.Stream(ctx, req, onToken)
	}
	if err := f.Primary.Stream(ctx, req, onToken); err != nil {
		if f.Fallback != nil {
			return f.Fallback.Stream(ctx, req, onToken)
		}
		return err
	}
	return nil
}
