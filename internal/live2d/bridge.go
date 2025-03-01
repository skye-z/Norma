package live2d

import (
	"norma/internal/companion"
	"norma/internal/runtime/bus"
)

// Bridge maps runtime events to Live2D events.
type Bridge struct {
	b *bus.Bus
}

func NewBridge(b *bus.Bus) *Bridge {
	br := &Bridge{b: b}
	br.bind()
	return br
}

func (br *Bridge) bind() {
	br.b.Subscribe("asr.final", func(evt bus.Event) {
		br.b.Publish(bus.Event{
			Name:      "live2d.motion",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"group": "thinking", "intensity": 0.5},
		})
	})

	br.b.Subscribe("live2d.emotion", func(evt bus.Event) {
		payload := normalizeEmotionPayload(evt.Payload)
		motion := payload["motion"]
		if motion == "" {
			motion = "Idle"
		}
		br.b.Publish(bus.Event{
			Name:      "live2d.motion",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"group": motion, "emotion": payload["emotion"], "intensity": payload["intensity"]},
		})
	})

	br.b.Subscribe("llm.token", func(evt bus.Event) {
		br.b.Publish(bus.Event{
			Name:      "live2d.mouth",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"level": 0.4, "mode": "token"},
		})
	})

	br.b.Subscribe("audio.play.end", func(evt bus.Event) {
		br.b.Publish(bus.Event{
			Name:      "live2d.mouth",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"level": 0, "mode": "idle"},
		})
	})

	br.b.Subscribe("llm.done", func(evt bus.Event) {
		br.b.Publish(bus.Event{
			Name:      "live2d.motion",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"group": "idle", "intensity": 0.3},
		})
		br.b.Publish(bus.Event{
			Name:      "live2d.mouth",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"level": 0, "mode": "idle"},
		})
	})
}

func normalizeEmotionPayload(value any) map[string]any {
	switch payload := value.(type) {
	case companion.EmotionSpec:
		return map[string]any{
			"emotion":   payload.Emotion,
			"motion":    payload.Motion,
			"intensity": payload.Intensity,
		}
	case *companion.EmotionSpec:
		if payload == nil {
			return map[string]any{}
		}
		return map[string]any{
			"emotion":   payload.Emotion,
			"motion":    payload.Motion,
			"intensity": payload.Intensity,
		}
	case map[string]any:
		return payload
	default:
		return map[string]any{}
	}
}
