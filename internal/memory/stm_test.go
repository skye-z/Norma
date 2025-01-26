package memory

import "testing"

func TestTrimToTokenBudget(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "短句"},
		{Role: "assistant", Content: "这是第一段稍长内容"},
		{Role: "user", Content: "这是第二段稍长内容"},
		{Role: "assistant", Content: "这是第三段稍长内容"},
	}

	out := TrimToTokenBudget(turns, 6)
	if len(out) == 0 {
		t.Fatalf("expected trimmed turns")
	}
	if out[len(out)-1].Content != "这是第三段稍长内容" {
		t.Fatalf("expected latest turn preserved")
	}
}
