package bus

import (
	"fmt"
	"sync"
	"time"
)

// Event is the runtime bus event payload.
type Event struct {
	Name      string `json:"name"`
	SessionID string `json:"session_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	Payload   any    `json:"payload,omitempty"`
	TsUnixMs  int64  `json:"ts_unix_ms"`
}

type Handler func(Event)

type subscription struct {
	id string
	h  Handler
}

// Bus is a minimal in-process pub/sub bus.
type Bus struct {
	mu        sync.RWMutex
	handlers  map[string][]subscription
	wildcards []subscription
	seq       uint64
}

func New() *Bus {
	return &Bus{handlers: make(map[string][]subscription)}
}

func (b *Bus) Publish(evt Event) {
	if evt.TsUnixMs == 0 {
		evt.TsUnixMs = time.Now().UnixMilli()
	}

	b.mu.RLock()
	subs := append([]subscription(nil), b.handlers[evt.Name]...)
	wildcards := append([]subscription(nil), b.wildcards...)
	b.mu.RUnlock()

	for _, sub := range subs {
		sub.h(evt)
	}
	for _, sub := range wildcards {
		sub.h(evt)
	}
}

func (b *Bus) Subscribe(name string, h Handler) func() {
	b.mu.Lock()
	b.seq++
	id := fmt.Sprintf("%s#%d", name, b.seq)
	b.handlers[name] = append(b.handlers[name], subscription{id: id, h: h})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.handlers[name]
		for i := range list {
			if list[i].id == id {
				b.handlers[name] = append(list[:i], list[i+1:]...)
				if len(b.handlers[name]) == 0 {
					delete(b.handlers, name)
				}
				return
			}
		}
	}
}

func (b *Bus) SubscribeAll(h Handler) func() {
	b.mu.Lock()
	b.seq++
	id := fmt.Sprintf("*#%d", b.seq)
	b.wildcards = append(b.wildcards, subscription{id: id, h: h})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i := range b.wildcards {
			if b.wildcards[i].id == id {
				b.wildcards = append(b.wildcards[:i], b.wildcards[i+1:]...)
				return
			}
		}
	}
}
