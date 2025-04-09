package state

import "testing"

func TestMachineLegalTransitions(t *testing.T) {
	m := NewMachine()
	m.Set(Listening)
	m.Set(Thinking)
	m.Set(Speaking)
	m.Set(Listening)

	if got := m.Get(); got != Listening {
		t.Fatalf("expected Listening, got %s", got)
	}

	h := m.History()
	if len(h) < 4 {
		t.Fatalf("expected history entries, got %d", len(h))
	}
	if !h[len(h)-1].Legal {
		t.Fatalf("expected last transition legal")
	}
}

func TestMachineIllegalTransitionIsRejected(t *testing.T) {
	m := NewMachine()
	m.Set(Speaking) // illegal from Idle

	if got := m.Get(); got != Idle {
		t.Fatalf("expected state remain Idle on illegal transition, got %s", got)
	}

	h := m.History()
	if len(h) == 0 {
		t.Fatalf("expected transition history")
	}
	last := h[len(h)-1]
	if last.Legal {
		t.Fatalf("expected illegal transition audit record")
	}
	if last.From != Idle || last.To != Speaking {
		t.Fatalf("unexpected transition record: %+v", last)
	}
}
