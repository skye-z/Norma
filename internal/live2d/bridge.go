package live2d

import "norma/internal/runtime/bus"

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
            Payload:   "thinking",
        })
    })

    br.b.Subscribe("llm.token", func(evt bus.Event) {
        br.b.Publish(bus.Event{
            Name:      "live2d.mouth",
            SessionID: evt.SessionID,
            TraceID:   evt.TraceID,
            Payload:   "speaking",
        })
    })

    br.b.Subscribe("llm.done", func(evt bus.Event) {
        br.b.Publish(bus.Event{
            Name:      "live2d.motion",
            SessionID: evt.SessionID,
            TraceID:   evt.TraceID,
            Payload:   "idle",
        })
    })
}
