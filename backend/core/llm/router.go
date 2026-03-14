// Package llm LLM 接口
package llm

import "context"

// 路由流式响应器
type RoutedStreamer struct {
	Primary      Streamer
	Degraded     Streamer
	Fallback     Streamer
	UseDegraded  func() bool
	CurrentRoute func() string
}

// 流式调用，自动路由
func (r RoutedStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	if r.UseDegraded != nil && r.UseDegraded() && r.Degraded != nil {
		if err := r.Degraded.Stream(ctx, req, onToken); err == nil {
			return nil
		}
	}
	if r.Primary != nil {
		if err := r.Primary.Stream(ctx, req, onToken); err == nil {
			return nil
		}
	}
	if r.Fallback != nil {
		return r.Fallback.Stream(ctx, req, onToken)
	}
	return nil
}
