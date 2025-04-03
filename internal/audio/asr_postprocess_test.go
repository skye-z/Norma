package audio

import "testing"

func TestPostProcessASRTextFixesWakeKeywordNearHomophone(t *testing.T) {
	got := PostProcessASRText("\u5f04\u561b,\u8acb\u4ecb\u7d39\u4e00\u4e0b\u4f60\u81ea\u5df1\u7684\u5f71\u7247\u3002", "\u8bfa\u739b")
	want := "\u8bfa\u739b,\u8bf7\u4ecb\u7ecd\u4e00\u4e0b\u4f60\u81ea\u5df1\u3002"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPostProcessASRTextKeepsCleanSentence(t *testing.T) {
	got := PostProcessASRText("\u8bfa\u739b\uff0c\u8bf7\u4ecb\u7ecd\u4e00\u4e0b\u4f60\u81ea\u5df1\u3002", "\u8bfa\u739b")
	want := "\u8bfa\u739b\uff0c\u8bf7\u4ecb\u7ecd\u4e00\u4e0b\u4f60\u81ea\u5df1\u3002"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPostProcessASRTextSupportsTraditionalWakeKeywordPrefix(t *testing.T) {
	got := PostProcessASRText("\u8afe\u746a\uff0c\u8acb\u4ecb\u7d39\u4e00\u4e0b\u4f60\u81ea\u5df1", "\u8bfa\u739b")
	want := "\u8bfa\u739b\uff0c\u8bf7\u4ecb\u7ecd\u4e00\u4e0b\u4f60\u81ea\u5df1"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
