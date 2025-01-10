package bus

import (
    "sync"
    "time"
)

// Event is the runtime bus event payload.
type Event struct {
    Name      string
    SessionID string
    TraceID   string
    Payload   any
    TsUnixMs  int64
}

type Handler func(Event)

// Bus is a minimal in-process pub/sub bus.
type Bus struct {
    mu       sync.RWMutex
    handlers map[string][]Handler
}

func New() *Bus {
    return &Bus{handlers: make(map[string][]Handler)}
}

func (b *Bus) Publish(evt Event) {
    if evt.TsUnixMs == 0 {
        evt.TsUnixMs = time.Now().UnixMilli()
    }

    b.mu.RLock()
    hs := append([]Handler(nil), b.handlers[evt.Name]...)
    b.mu.RUnlock()

    for _, h := range hs {
        h(evt)
    }
}

func (b *Bus) Subscribe(name string, h Handler) func() {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[name] = append(b.handlers[name], h)
    idx := len(b.handlers[name]) - 1

    return func() {
        b.mu.Lock()
        defer b.mu.Unlock()
        list := b.handlers[name]
        if idx < 0 || idx >= len(list) {
            return
        }
        b.handlers[name] = append(list[:idx], list[idx+1:]...)
    }
}
