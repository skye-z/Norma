package runtime

import (
	"log"

	"norma/backend/core/runtime/bus"
	"norma/backend/core/runtime/state"
)

// 运行时核心组件
type Runtime struct {
	Bus       *bus.Bus       // 事件总线
	State     *state.Machine // 状态机
	Inspector *Inspector     // 诊断
}

// 创建运行时实例
func New() *Runtime {
	inspector := NewInspector(80)
	rt := &Runtime{
		Bus:       bus.New(),
		State:     state.NewMachine(),
		Inspector: inspector,
	}
	_ = inspector.Bind(rt.Bus)
	inspector.SetState(string(rt.State.Get()))

	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		if rt.State.Get() == state.Speaking {
			log.Printf("[runtime] barge-in trigger while speaking session=%s trace=%s", evt.SessionID, evt.TraceID)
			rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: "barge_in"})
		}
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] audio.vad.start session=%s trace=%s", evt.SessionID, evt.TraceID)
	})

	rt.Bus.Subscribe("asr.partial", func(evt bus.Event) {
		if text, ok := evt.Payload.(string); ok {
			log.Printf("[runtime] asr.partial session=%s trace=%s text=%q", evt.SessionID, evt.TraceID, text)
		}
		_ = evt
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
	})

	rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
		rt.State.Set(state.Thinking)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] asr.final -> Thinking session=%s trace=%s text=%v", evt.SessionID, evt.TraceID, evt.Payload)
	})

	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		if text, ok := evt.Payload.(string); ok {
			log.Printf("[runtime] llm.token session=%s trace=%s token=%q", evt.SessionID, evt.TraceID, text)
		}
		_ = evt
		rt.State.Set(state.Speaking)
		inspector.SetState(string(rt.State.Get()))
	})

	for _, name := range []string{"llm.done", "session.done"} {
		rt.Bus.Subscribe(name, func(evt bus.Event) {
			_ = evt
			rt.State.Set(state.Listening)
			inspector.SetState(string(rt.State.Get()))
		})
	}

	rt.Bus.Subscribe("interrupt", func(evt bus.Event) {
		rt.State.Set(state.Interrupted)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] interrupt received: session=%s trace=%s", evt.SessionID, evt.TraceID)
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
	})

	rt.Bus.Subscribe("error", func(evt bus.Event) {
		rt.State.Set(state.Error)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] error event: session=%s trace=%s payload=%v", evt.SessionID, evt.TraceID, evt.Payload)
		rt.State.Set(state.Idle)
		inspector.SetState(string(rt.State.Get()))
	})

	return rt
}
