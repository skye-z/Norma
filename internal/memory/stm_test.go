package memory

import "testing"

func TestTrimToTokenBudget(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "短句", Importance: 1},
		{Role: "assistant", Content: "这是第一段稍长内容", Importance: 1},
		{Role: "user", Content: "这是第二段稍长内容", Importance: 3, Pinned: true},
		{Role: "assistant", Content: "这是第三段稍长内容", Importance: 2},
	}

	out := TrimToTokenBudget(turns, 6)
	if len(out) == 0 {
		t.Fatalf("expected trimmed turns")
	}
	foundPinned := false
	for _, turn := range out {
		if turn.Content == "这是第二段稍长内容" {
			foundPinned = true
		}
	}
	if !foundPinned {
		t.Fatalf("expected important pinned turn preserved")
	}
	if out[len(out)-1].Content != "这是第三段稍长内容" {
		t.Fatalf("expected latest turn preserved")
	}
}
