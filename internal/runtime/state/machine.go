package state

import "sync"

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

// Machine is a threadsafe state holder.
type Machine struct {
    mu    sync.RWMutex
    state State
}

func NewMachine() *Machine {
    return &Machine{state: Idle}
}

func (m *Machine) Get() State {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.state
}

func (m *Machine) Set(next State) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.state = next
}
