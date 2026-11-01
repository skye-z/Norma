package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownStoreWriteReadAppendRemove(t *testing.T) {
	s := NewMarkdownStore(filepath.Join(t.TempDir(), "mem", "profile.md"), 64)
	if got, err := s.Read(); err != nil || got != "" {
		t.Fatalf("empty read = %q, %v", got, err)
	}
	if err := s.Append("我喜欢喝美式咖啡"); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("我喜欢喝美式咖啡"); err != nil { // 重复写入应被忽略
		t.Fatal(err)
	}
	if err := s.Append("我住在深圳"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Read()
	if strings.Count(got, "美式咖啡") != 1 || !strings.Contains(got, "深圳") {
		t.Fatalf("unexpected body %q", got)
	}
	n, err := s.Remove("深圳")
	if err != nil || n != 1 {
		t.Fatalf("remove = %d, %v", n, err)
	}
	if got, _ = s.Read(); strings.Contains(got, "深圳") {
		t.Fatalf("removed item still present: %q", got)
	}
}

func TestMarkdownStoreRejectsOversizeAndBadUTF8KeepingOldFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.md")
	s := NewMarkdownStore(path, 16)
	if err := s.Write("- 短内容\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(strings.Repeat("长", 20)); err == nil {
		t.Fatal("expected size error")
	}
	if err := s.Write("bad\xff"); err == nil {
		t.Fatal("expected encoding error")
	}
	if b, _ := os.ReadFile(path); string(b) != "- 短内容\n" {
		t.Fatalf("original file damaged: %q", b)
	}
	if err := os.WriteFile(path, []byte("\xff\xfe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(); err != ErrMarkdownBadEncoding {
		t.Fatalf("expected bad encoding on read, got %v", err)
	}
}

func TestMarkdownRetrieverReturnsWholeBodyEveryTurn(t *testing.T) {
	s := NewMarkdownStore(filepath.Join(t.TempDir(), "p.md"), 0)
	r := MarkdownRetriever{Store: s}
	if hits, _ := r.Search(context.Background(), "q", 3); len(hits) != 0 {
		t.Fatal("expected no hits for empty file")
	}
	_ = s.Append("我对花生过敏")
	hits, _ := r.Search(context.Background(), "无关问题", 3)
	if len(hits) != 1 || !strings.Contains(hits[0].Content, "花生") {
		t.Fatalf("unexpected hits %#v", hits)
	}
	_, _ = s.Remove("花生")
	if hits, _ = r.Search(context.Background(), "q", 3); len(hits) != 0 && strings.Contains(hits[0].Content, "花生") {
		t.Fatal("deleted item must not be loaded again")
	}
}

func TestParseRememberCommand(t *testing.T) {
	cases := map[string]string{"记住我喜欢喝美式。": "我喜欢喝美式", "请记住，我的生日是三月五日": "我的生日是三月五日"}
	for in, want := range cases {
		if got, ok := ParseRememberCommand(in); !ok || got != want {
			t.Errorf("%q => %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := ParseRememberCommand("你还记得我吗"); ok {
		t.Error("question must not be a remember command")
	}
}

func TestParseRememberCommandRejectsQuestionsAndFillers(t *testing.T) {
	for _, in := range []string{"你记住了吗？", "你记住了吗", "记住了", "你能记住我喜欢咖啡吗", "记住"} {
		if fact, ok := ParseRememberCommand(in); ok {
			t.Errorf("%q should not be a remember command, got fact %q", in, fact)
		}
	}
	if fact, ok := ParseRememberCommand("记住我喜欢喝美式"); !ok || fact != "我喜欢喝美式" {
		t.Errorf("valid command rejected: %q %v", fact, ok)
	}
}
