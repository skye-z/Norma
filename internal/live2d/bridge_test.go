package live2d

import (
	"testing"
	"time"

	"norma/internal/companion"
	"norma/internal/runtime/bus"
)

func TestBridgeEmotionStructPayload(t *testing.T) {
	b := bus.New()
	_ = NewBridge(b)

	ch := make(chan bus.Event, 1)
	b.Subscribe("live2d.motion", func(evt bus.Event) {
		if payload, _ := evt.Payload.(map[string]any); payload["emotion"] == "guide" {
			ch <- evt
		}
	})

	b.Publish(bus.Event{
		Name:      "live2d.emotion",
		SessionID: "s1",
		TraceID:   "t1",
		Payload: companion.EmotionSpec{
			Emotion:   "guide",
			Motion:    "Tap",
			Intensity: 0.6,
		},
	})

	select {
	case evt := <-ch:
		payload, _ := evt.Payload.(map[string]any)
		if payload["group"] != "Tap" {
			t.Fatalf("unexpected motion payload: %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live2d.motion event")
	}
}
