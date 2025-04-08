package live2d

import (
	"sync"
	"time"

	"norma/internal/companion"
	"norma/internal/runtime/bus"
)

// Bridge maps runtime events to Live2D events.
type Bridge struct {
	b     *bus.Bus
	mu    sync.Mutex
	state map[string]motionState
}

type motionState struct {
	Group       string
	Priority    int
	CooldownTo  time.Time
	IdleAfterMs int
}

func NewBridge(b *bus.Bus) *Bridge {
	br := &Bridge{b: b, state: make(map[string]motionState)}
	br.bind()
	return br
}

func (br *Bridge) bind() {
	br.b.Subscribe("asr.final", func(evt bus.Event) {
		br.publishScheduledMotion(evt, map[string]any{"group": "thinking", "intensity": 0.5, "priority": 70, "cooldown_ms": 250, "interruptible": true, "idle_after_ms": 900})
	})

	br.b.Subscribe("live2d.emotion", func(evt bus.Event) {
		payload := normalizeEmotionPayload(evt.Payload)
		motion := payload["motion"]
		if motion == "" {
			motion = "Idle"
		}
		br.publishScheduledMotion(evt, map[string]any{"group": motion, "emotion": payload["emotion"], "intensity": payload["intensity"], "priority": 80, "cooldown_ms": 600, "interruptible": true, "idle_after_ms": 1400})
		if expression := payload["expression"]; expression != "" {
			br.b.Publish(bus.Event{Name: "live2d.expression", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: map[string]any{"name": expression, "emotion": payload["emotion"], "intensity": payload["intensity"], "hold_ms": 1400}})
		}
		if params, ok := payload["params"].(map[string]float64); ok && len(params) > 0 {
			br.b.Publish(bus.Event{Name: "live2d.parameters", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: map[string]any{"params": params, "blend_ms": 900, "hold_ms": 1400}})
		}
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
		br.publishScheduledMotion(evt, map[string]any{"group": "idle", "intensity": 0.3, "priority": 10, "cooldown_ms": 0, "interruptible": true, "idle_after_ms": 0, "force": true})
		br.b.Publish(bus.Event{
			Name:      "live2d.mouth",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"level": 0, "mode": "idle"},
		})
	})
}

func (br *Bridge) publishScheduledMotion(evt bus.Event, payload map[string]any) {
	group, _ := payload["group"].(string)
	priority, _ := payload["priority"].(int)
	if priority == 0 {
		priority = 50
	}
	cooldownMs, _ := payload["cooldown_ms"].(int)
	force, _ := payload["force"].(bool)

	br.mu.Lock()
	current := br.state[evt.SessionID]
	now := time.Now()
	if !force {
		if now.Before(current.CooldownTo) && priority < current.Priority {
			br.mu.Unlock()
			return
		}
		if group == current.Group && now.Before(current.CooldownTo) {
			br.mu.Unlock()
			return
		}
	}
	current = motionState{Group: group, Priority: priority}
	if cooldownMs > 0 {
		current.CooldownTo = now.Add(time.Duration(cooldownMs) * time.Millisecond)
	}
	if idleAfter, ok := payload["idle_after_ms"].(int); ok {
		current.IdleAfterMs = idleAfter
	}
	br.state[evt.SessionID] = current
	br.mu.Unlock()

	br.b.Publish(bus.Event{Name: "live2d.motion", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: payload})

	if current.IdleAfterMs > 0 && group != "idle" {
		go br.scheduleIdle(evt.SessionID, evt.TraceID, current.IdleAfterMs, priority)
	}
}

func (br *Bridge) scheduleIdle(sessionID, traceID string, delayMs int, parentPriority int) {
	if delayMs <= 0 {
		return
	}
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
	br.mu.Lock()
	current := br.state[sessionID]
	if current.Group == "idle" || current.Priority > parentPriority {
		br.mu.Unlock()
		return
	}
	br.state[sessionID] = motionState{Group: "idle", Priority: 10}
	br.mu.Unlock()
	br.b.Publish(bus.Event{Name: "live2d.motion", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"group": "idle", "intensity": 0.28, "priority": 10, "interruptible": true, "reason": "idle_recovery"}})
}

func normalizeEmotionPayload(value any) map[string]any {
	switch payload := value.(type) {
	case companion.EmotionSpec:
		return map[string]any{
			"emotion":    payload.Emotion,
			"motion":     payload.Motion,
			"expression": payload.Expression,
			"intensity":  payload.Intensity,
			"params":     payload.Params,
		}
	case *companion.EmotionSpec:
		if payload == nil {
			return map[string]any{}
		}
		return map[string]any{
			"emotion":    payload.Emotion,
			"motion":     payload.Motion,
			"expression": payload.Expression,
			"intensity":  payload.Intensity,
			"params":     payload.Params,
		}
	case map[string]any:
		return payload
	default:
		return map[string]any{}
	}
}
