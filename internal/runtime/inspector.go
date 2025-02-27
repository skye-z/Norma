package runtime

import (
	"sync"

	"norma/internal/runtime/bus"
)

// DiagnosticSnapshot exposes the latest runtime state for the dev panel.
type DiagnosticSnapshot struct {
	State      string      `json:"state"`
	QueueDepth int         `json:"queue_depth"`
	LastEvent  string      `json:"last_event"`
	Events     []bus.Event `json:"events"`
	Sessions   []string    `json:"sessions"`
}

// Inspector records recent runtime activity for debugging.
type Inspector struct {
	mu         sync.RWMutex
	maxEvents  int
	state      string
	queueDepth int
	lastEvent  string
	events     []bus.Event
	sessions   map[string]struct{}
}

func NewInspector(maxEvents int) *Inspector {
	if maxEvents <= 0 {
		maxEvents = 64
	}
	return &Inspector{maxEvents: maxEvents, sessions: make(map[string]struct{})}
}

func (i *Inspector) Bind(b *bus.Bus) func() {
	if b == nil {
		return func() {}
	}
	return b.SubscribeAll(func(evt bus.Event) {
		i.Record(evt)
	})
}

func (i *Inspector) Record(evt bus.Event) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if evt.SessionID != "" {
		i.sessions[evt.SessionID] = struct{}{}
	}
	i.lastEvent = evt.Name
	i.events = append(i.events, evt)
	if len(i.events) > i.maxEvents {
		i.events = append([]bus.Event(nil), i.events[len(i.events)-i.maxEvents:]...)
	}
}

func (i *Inspector) SetState(state string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.state = state
}

func (i *Inspector) SetQueueDepth(depth int) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.queueDepth = depth
}

func (i *Inspector) Snapshot() DiagnosticSnapshot {
	i.mu.RLock()
	defer i.mu.RUnlock()
	events := append([]bus.Event(nil), i.events...)
	sessions := make([]string, 0, len(i.sessions))
	for sessionID := range i.sessions {
		sessions = append(sessions, sessionID)
	}
	return DiagnosticSnapshot{
		State:      i.state,
		QueueDepth: i.queueDepth,
		LastEvent:  i.lastEvent,
		Events:     events,
		Sessions:   sessions,
	}
}
