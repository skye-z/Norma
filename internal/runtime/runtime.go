package runtime

import (
	"log"

	"norma/internal/runtime/bus"
	"norma/internal/runtime/state"
)

// Runtime wires core runtime components.
type Runtime struct {
	Bus       *bus.Bus
	State     *state.Machine
	Inspector *Inspector
}

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
		_ = evt
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
	})

	rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
		rt.State.Set(state.Thinking)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] asr.final -> Thinking session=%s trace=%s", evt.SessionID, evt.TraceID)
	})

	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		_ = evt
		rt.State.Set(state.Speaking)
		inspector.SetState(string(rt.State.Get()))
	})

	rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		_ = evt
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
	})

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
