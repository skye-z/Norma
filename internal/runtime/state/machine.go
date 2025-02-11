package state

import (
	"sync"
	"time"
)

// State is the global runtime state.
type State string

const (
	Idle        State = "Idle"
	Listening   State = "Listening"
	Thinking    State = "Thinking"
	Speaking    State = "Speaking"
	Interrupted State = "Interrupted"
	Error       State = "Error"
)

// Transition keeps auditable state movement.
type Transition struct {
	From     State
	To       State
	TsUnixMs int64
	Legal    bool
}

const historyLimit = 256

// Machine is a threadsafe state holder.
type Machine struct {
	mu      sync.RWMutex
	state   State
	history []Transition
}

func NewMachine() *Machine {
	return &Machine{state: Idle}
}

func (m *Machine) Get() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Set applies a transition if legal, and records an audit trail.
func (m *Machine) Set(next State) {
	m.mu.Lock()
	defer m.mu.Unlock()

	from := m.state
	legal := isLegalTransition(from, next)
	m.appendHistory(Transition{
		From:     from,
		To:       next,
		TsUnixMs: time.Now().UnixMilli(),
		Legal:    legal,
	})

	if legal {
		m.state = next
	}
}

// History returns a copy of transition audit logs.
func (m *Machine) History() []Transition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Transition, len(m.history))
	copy(out, m.history)
	return out
}

func (m *Machine) appendHistory(t Transition) {
	m.history = append(m.history, t)
	if len(m.history) > historyLimit {
		m.history = append([]Transition(nil), m.history[len(m.history)-historyLimit:]...)
	}
}

func isLegalTransition(from, to State) bool {
	if to == "" {
		return false
	}
	if from == to {
		return true
	}

	// Global escape branches in architecture docs.
	if to == Interrupted || to == Error {
		return true
	}

	switch from {
	case Idle:
		return to == Listening || to == Thinking
	case Listening:
		return to == Thinking || to == Idle
	case Thinking:
		return to == Speaking || to == Listening
	case Speaking:
		return to == Listening || to == Thinking
	case Interrupted:
		return to == Listening || to == Idle
	case Error:
		return to == Idle || to == Listening
	default:
		return false
	}
}
