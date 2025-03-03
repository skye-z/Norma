package audio

import (
	"fmt"
	"testing"
)

func TestSherpaWakeMatched(t *testing.T) {
	if !sherpaWakeMatched(`{"keyword":"诺玛"}`, "诺玛") {
		t.Fatalf("expected keyword match")
	}
	if sherpaWakeMatched(`{"keyword":"其他"}`, "诺玛") {
		t.Fatalf("unexpected keyword match")
	}
}

func TestSherpaWakeWordDetectsFromFrames(t *testing.T) {
	wake := NewSherpaWakeWord("kw.exe", "encoder.onnx", "decoder.onnx", "joiner.onnx", "tokens.txt", "keywords.txt", "诺玛")
	wake.WindowFrames = 4
	wake.DetectInterval = 1
	wake.commandRunner = func(args []string) (string, error) {
		return fmt.Sprintf(`{"keyword":"%s"}`, "诺玛"), nil
	}
	frame := make([]int16, 160)
	for i := range frame {
		frame[i] = 2048
	}
	matched := false
	for i := 0; i < 4; i++ {
		ok, err := wake.Detect(frame)
		if err != nil {
			t.Fatalf("detect: %v", err)
		}
		matched = matched || ok
	}
	if !matched {
		t.Fatalf("expected wake match")
	}
}
