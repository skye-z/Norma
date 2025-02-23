package llm

import "context"

// FallbackStreamer tries primary first, then fallback on error.
type FallbackStreamer struct {
	Primary  Streamer
	Fallback Streamer
}

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
