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

func TestPostProcessASRTextFixesIntroSentenceNearMiss(t *testing.T) {
	got := PostProcessASRText("\u8c22\u8c22\u5927\u5bb6\u6536\u770b!", "\u8bfa\u739b")
	want := "\u8bf7\u4ecb\u7ecd\u4e00\u4e0b\u4f60\u81ea\u5df1!"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPostProcessASRTextFixesTruncatedIntroSentence(t *testing.T) {
	got := PostProcessASRText("\u8bfa\u9a6c\u8bf7\u4ecb\u7ecd\u4e00\u4e0b", "\u8bfa\u739b")
	want := "\u8bfa\u739b\u8bf7\u4ecb\u7ecd\u4e00\u4e0b\u4f60\u81ea\u5df1"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPostProcessASRTextFixesWeatherQueryNearMiss(t *testing.T) {
	got := PostProcessASRText("\u90a3\u4e48\u591a\u6570\u5b57\u5e55 \u8c22\u8c22\u89c2\u770b", "\u8bfa\u739b")
	want := "\u8bfa\u739b\u4eca\u5929\u5929\u6c14\u600e\u4e48\u6837\u5e2e\u6211\u67e5\u4e00\u4e0b"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
