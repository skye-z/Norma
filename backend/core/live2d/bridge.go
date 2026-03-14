// Package live2d Live2D虚拟形象事件桥接
package live2d

import (
	"sync"
	"time"

	"norma/backend/core/companion"
	"norma/backend/core/runtime/bus"
)

// 运行时事件到Live2D事件的桥接器
type Bridge struct {
	b     *bus.Bus // 事件总线
	mu    sync.Mutex
	state map[string]motionState // 会话ID -> 动作状态
}

// motionState 会话动作状态
type motionState struct {
	Group       string    // 动作组名称
	Priority    int       // 优先级，数值越大优先级越高
	CooldownTo  time.Time // 冷却截止时间
	IdleAfterMs int       // 空闲恢复延迟（毫秒）
}

// 创建Live2D桥接器
func NewBridge(b *bus.Bus) *Bridge {
	br := &Bridge{b: b, state: make(map[string]motionState)}
	br.bind()
	return br
}

// bind 绑定事件订阅
func (br *Bridge) bind() {
	// ASR识别完成时，触发思考动作
	br.b.Subscribe("asr.final", func(evt bus.Event) {
		br.publishScheduledMotion(evt, map[string]any{"group": "thinking", "intensity": 0.5, "priority": 70, "cooldown_ms": 250, "interruptible": true, "idle_after_ms": 900})
	})

	// 情感事件时，触发相应的动作、表情和参数
	br.b.Subscribe("live2d.emotion", func(evt bus.Event) {
		payload := normalizeEmotionPayload(evt.Payload)
		motion := payload["motion"]
		if motion == "" {
			motion = "Idle"
		}
		// 发布动作事件
		br.publishScheduledMotion(evt, map[string]any{"group": motion, "emotion": payload["emotion"], "intensity": payload["intensity"], "priority": 80, "cooldown_ms": 600, "interruptible": true, "idle_after_ms": 1400})
		// 发布表情事件（如果存在）
		if expression := payload["expression"]; expression != "" {
			br.b.Publish(bus.Event{Name: "live2d.expression", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: map[string]any{"name": expression, "emotion": payload["emotion"], "intensity": payload["intensity"], "hold_ms": 1400}})
		}
		// 发布模型参数事件（如果存在）
		if params, ok := payload["params"].(map[string]float64); ok && len(params) > 0 {
			br.b.Publish(bus.Event{Name: "live2d.parameters", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: map[string]any{"params": params, "blend_ms": 900, "hold_ms": 1400}})
		}
	})

	// LLM生成token时，触发言语动作（嘴型）
	br.b.Subscribe("llm.token", func(evt bus.Event) {
		br.b.Publish(bus.Event{
			Name:      "live2d.mouth",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"level": 0.4, "mode": "token"},
		})
	})

	// 音频播放结束时，恢复嘴型到空闲状态
	br.b.Subscribe("audio.play.end", func(evt bus.Event) {
		br.b.Publish(bus.Event{
			Name:      "live2d.mouth",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   map[string]any{"level": 0, "mode": "idle"},
		})
	})

	// LLM响应完成时，触发空闲动作
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

// publishScheduledMotion 发布调度动作事件
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
	// 非强制模式下，检查优先级和冷却时间
	if !force {
		// 如果当前动作还在冷却中且新动作优先级更低，跳过
		if now.Before(current.CooldownTo) && priority < current.Priority {
			br.mu.Unlock()
			return
		}
		// 如果是同一动作组且在冷却中，跳过
		if group == current.Group && now.Before(current.CooldownTo) {
			br.mu.Unlock()
			return
		}
	}
	// 更新动作状态
	current = motionState{Group: group, Priority: priority}
	if cooldownMs > 0 {
		current.CooldownTo = now.Add(time.Duration(cooldownMs) * time.Millisecond)
	}
	if idleAfter, ok := payload["idle_after_ms"].(int); ok {
		current.IdleAfterMs = idleAfter
	}
	br.state[evt.SessionID] = current
	br.mu.Unlock()

	// 发布动作事件
	br.b.Publish(bus.Event{Name: "live2d.motion", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: payload})

	// 如果需要空闲恢复，调度空闲动作
	if current.IdleAfterMs > 0 && group != "idle" {
		go br.scheduleIdle(evt.SessionID, evt.TraceID, current.IdleAfterMs, priority)
	}
}

// scheduleIdle 调度空闲恢复动作
func (br *Bridge) scheduleIdle(sessionID, traceID string, delayMs int, parentPriority int) {
	if delayMs <= 0 {
		return
	}
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
	br.mu.Lock()
	current := br.state[sessionID]
	// 如果已经切换到idle或被更高优先级动作取代，则跳过
	if current.Group == "idle" || current.Priority > parentPriority {
		br.mu.Unlock()
		return
	}
	br.state[sessionID] = motionState{Group: "idle", Priority: 10}
	br.mu.Unlock()
	br.b.Publish(bus.Event{Name: "live2d.motion", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"group": "idle", "intensity": 0.28, "priority": 10, "interruptible": true, "reason": "idle_recovery"}})
}

// normalizeEmotionPayload 标准化情感负载
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
