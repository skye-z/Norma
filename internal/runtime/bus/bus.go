package bus

import (
	"fmt"
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

type subscription struct {
	id string
	h  Handler
}

// Bus is a minimal in-process pub/sub bus.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]subscription
	seq      uint64
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
	b.mu.RUnlock()

	for _, sub := range subs {
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
