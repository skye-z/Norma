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
