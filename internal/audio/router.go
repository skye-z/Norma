package audio

import "context"

type RoutedTTS struct {
	Primary     TTS
	Degraded    TTS
	Fallback    TTS
	UseDegraded func() bool
}

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

func (r *RoutedTTS) SampleRate() int {
	if provider, ok := r.selected().(AudioFormatProvider); ok {
		return provider.SampleRate()
	}
	return 22050
}

func (r *RoutedTTS) Channels() int {
	if provider, ok := r.selected().(AudioFormatProvider); ok {
		return provider.Channels()
	}
	return 1
}

func (r *RoutedTTS) Encoding() string {
	if provider, ok := r.selected().(AudioFormatProvider); ok {
		return provider.Encoding()
	}
	return "pcm_s16le"
}

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
