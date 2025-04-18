// Package audio 音频处理
package audio

import "context"

// RoutedTTS 路由 TTS
type RoutedTTS struct {
	Primary     TTS
	Degraded    TTS
	Fallback    TTS
	UseDegraded func() bool
}

// Stream 流式合成
func (r *RoutedTTS) Stream(ctx context.Context, text string, onChunk func([]byte)) error {
	if r != nil && r.UseDegraded != nil && r.UseDegraded() && r.Degraded != nil {
		if err := r.Degraded.Stream(ctx, text, onChunk); err == nil {
			return nil
		}
	}
	if r != nil && r.Primary != nil {
		if err := r.Primary.Stream(ctx, text, onChunk); err == nil {
			return nil
		}
	}
	if r != nil && r.Fallback != nil {
		return r.Fallback.Stream(ctx, text, onChunk)
	}
	return nil
}

// Stop 停止播放
func (r *RoutedTTS) Stop(sessionID string) error {
	if r == nil {
		return nil
	}
	if r.Primary != nil {
		_ = r.Primary.Stop(sessionID)
	}
	if r.Degraded != nil {
		_ = r.Degraded.Stop(sessionID)
	}
	if r.Fallback != nil {
		_ = r.Fallback.Stop(sessionID)
	}
	return nil
}

// SampleRate 获取采样率
func (r *RoutedTTS) SampleRate() int {
	if provider, ok := r.selected().(AudioFormatProvider); ok {
		return provider.SampleRate()
	}
	return 22050
}

// Channels 获取声道数
func (r *RoutedTTS) Channels() int {
	if provider, ok := r.selected().(AudioFormatProvider); ok {
		return provider.Channels()
	}
	return 1
}

// Encoding 获取编码格式
func (r *RoutedTTS) Encoding() string {
	if provider, ok := r.selected().(AudioFormatProvider); ok {
		return provider.Encoding()
	}
	return "pcm_s16le"
}

// selected 获取当前选中的 TTS
func (r *RoutedTTS) selected() TTS {
	if r == nil {
		return nil
	}
	if r.UseDegraded != nil && r.UseDegraded() && r.Degraded != nil {
		return r.Degraded
	}
	if r.Primary != nil {
		return r.Primary
	}
	return r.Fallback
}

// SupportsProsodyHints 是否支持韵律提示
func (r *RoutedTTS) SupportsProsodyHints() bool {
	if provider, ok := r.selected().(ProsodyHintProvider); ok {
		return provider.SupportsProsodyHints()
	}
	return false
}
