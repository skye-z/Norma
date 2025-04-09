package audio

import "testing"

func TestSplitSentences(t *testing.T) {
	in := "你好。今天天气不错！要继续开发吗？可以。"
	got := splitSentences(in)

	if len(got) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(got))
	}
	if got[0] != "你好。" || got[1] != "今天天气不错！" || got[2] != "要继续开发吗？" || got[3] != "可以。" {
		t.Fatalf("unexpected split result: %#v", got)
	}
}

func TestPrioritizeSentenceParts(t *testing.T) {
	parts := prioritizeSentenceParts([]string{"这是一段比较长的说明，需要稍后播报。", "好的。", "收到。"})
	if len(parts) != 3 {
		t.Fatalf("unexpected parts len: %d", len(parts))
	}
	if parts[0] != "好的。" || parts[1] != "收到。" {
		t.Fatalf("expected short sentences first, got %#v", parts)
	}
}
