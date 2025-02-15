package runtime

import (
	"log"

	"norma/internal/runtime/bus"
	"norma/internal/runtime/state"
)

// Runtime wires core runtime components.
type Runtime struct {
	Bus   *bus.Bus
	State *state.Machine
}

func New() *Runtime {
	rt := &Runtime{
		Bus:   bus.New(),
		State: state.NewMachine(),
	}

	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		rt.State.Set(state.Listening)
		log.Printf("[runtime] audio.vad.start session=%s trace=%s", evt.SessionID, evt.TraceID)
	})

	rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
		rt.State.Set(state.Thinking)
		log.Printf("[runtime] asr.final -> Thinking session=%s trace=%s", evt.SessionID, evt.TraceID)
	})

	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		rt.State.Set(state.Speaking)
	})

	rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		_ = evt
		rt.State.Set(state.Listening)
	})

	rt.Bus.Subscribe("interrupt", func(evt bus.Event) {
		rt.State.Set(state.Interrupted)
		log.Printf("[runtime] interrupt received: session=%s trace=%s", evt.SessionID, evt.TraceID)
		rt.State.Set(state.Listening)
	})

	rt.Bus.Subscribe("error", func(evt bus.Event) {
		rt.State.Set(state.Error)
		log.Printf("[runtime] error event: session=%s trace=%s payload=%v", evt.SessionID, evt.TraceID, evt.Payload)
		rt.State.Set(state.Idle)
	})

	return rt
}
