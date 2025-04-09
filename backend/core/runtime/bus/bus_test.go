package bus

import "testing"

func TestUnsubscribeOrderStable(t *testing.T) {
	b := New()

	countA := 0
	countB := 0
	unsubA := b.Subscribe("demo", func(Event) { countA++ })
	unsubB := b.Subscribe("demo", func(Event) { countB++ })

	unsubA()
	b.Publish(Event{Name: "demo"})

	if countA != 0 {
		t.Fatalf("subscriber A should be removed, got count=%d", countA)
	}
	if countB != 1 {
		t.Fatalf("subscriber B should still receive event, got count=%d", countB)
	}

	unsubB()
	b.Publish(Event{Name: "demo"})
	if countB != 1 {
		t.Fatalf("subscriber B should be removed, got count=%d", countB)
	}
}

func TestEventTimestampAutoFill(t *testing.T) {
	b := New()
	gotTs := int64(0)
	_ = b.Subscribe("demo", func(evt Event) {
		gotTs = evt.TsUnixMs
	})

	b.Publish(Event{Name: "demo"})
	if gotTs <= 0 {
		t.Fatalf("expected TsUnixMs to be auto-filled, got %d", gotTs)
	}
}
