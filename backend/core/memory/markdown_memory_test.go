package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingIndex struct{ got map[string][]string }

func (r *recordingIndex) Upsert(path string, chunks []string) error {
	if r.got == nil {
		r.got = map[string][]string{}
	}
	r.got[path] = append([]string(nil), chunks...)
	return nil
}

func TestParseMarkdownEntriesOnlyListItems(t *testing.T) {
	body := "# 记忆\n\n- 喜欢喝美式\n* 养了一只猫\n普通段落不算\n-  \n- 周末爬山\n"
	got := ParseMarkdownEntries(body)
	want := []string{"喜欢喝美式", "养了一只猫", "周末爬山"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMarkdownMemoryRememberSyncsIndexes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.md")
	a, b := &recordingIndex{}, &recordingIndex{}
	m := NewMarkdownMemory(NewMarkdownStore(path, 0), a, b, nil)
	if err := m.Remember(context.Background(), "我喜欢喝美式咖啡"); err != nil {
		t.Fatal(err)
	}
	if err := m.Remember(context.Background(), "我喜欢喝美式咖啡"); err != nil { // 重复不应产生第二条
		t.Fatal(err)
	}
	if err := m.Remember(context.Background(), "猫叫团子"); err != nil {
		t.Fatal(err)
	}
	for _, ix := range []*recordingIndex{a, b} {
		got := ix.got[markdownDocPath]
		if len(got) != 2 || got[0] != "我喜欢喝美式咖啡" || got[1] != "猫叫团子" {
			t.Fatalf("index entries = %v", got)
		}
	}
	body, _ := os.ReadFile(path)
	if strings.Count(string(body), "美式") != 1 {
		t.Fatalf("file content: %q", body)
	}
}

func TestMarkdownMemoryRememberHonorsCanceledContext(t *testing.T) {
	m := NewMarkdownMemory(NewMarkdownStore(filepath.Join(t.TempDir(), "m.md"), 0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Remember(ctx, "x"); err == nil {
		t.Fatal("expected context error")
	}
}

func TestMarkdownMemoryEndToEndWithRealIndexes(t *testing.T) {
	dir := t.TempDir()
	vec, err := NewDiskVectorStore(filepath.Join(dir, "v.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewMarkdownMemory(NewMarkdownStore(filepath.Join(dir, "m.md"), 0), vec)
	if err := m.Remember(context.Background(), "我的猫叫团子"); err != nil {
		t.Fatal(err)
	}
	hits, err := vec.Search(context.Background(), "猫叫什么", 3)
	if err != nil || len(hits) == 0 || hits[0].Content != "我的猫叫团子" {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
}
