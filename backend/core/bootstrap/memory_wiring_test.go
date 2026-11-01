package bootstrap

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"norma/backend/core/db"
	"norma/backend/core/memory"
	"norma/backend/core/providers"
	config "norma/backend/utils"
)

func memoryTestConfig(dir string) *config.Config {
	cfg := &config.Config{}
	cfg.RAG.Provider = "hybrid"
	cfg.RAG.VectorIndexPath = filepath.Join(dir, "vec.json")
	cfg.Memory.MarkdownPath = filepath.Join(dir, "memory.md")
	return cfg
}

// 记住一条事实后能被检索到；重新构建检索器（模拟重启）后仍然存在
func TestRememberedFactSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := memoryTestConfig(dir)
	mgr := providers.NewManager(filepath.Join(dir, "providers"))

	engine, err := db.NewEngine(filepath.Join(dir, "m.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	retriever, md := chooseRetriever(cfg, engine, mgr)
	if md == nil {
		t.Fatal("markdown memory is nil")
	}
	if err := md.Remember(context.Background(), "我的猫叫团子"); err != nil {
		t.Fatal(err)
	}
	if !searchFinds(t, retriever, "猫叫团子", "我的猫叫团子") {
		t.Fatal("fact not retrievable right after remember")
	}

	// 新的数据库文件模拟索引丢失，只有 memory.md 保留
	engine2, err := db.NewEngine(filepath.Join(dir, "m2.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	retriever2, _ := chooseRetriever(cfg, engine2, mgr)
	if !searchFinds(t, retriever2, "猫叫团子", "我的猫叫团子") {
		t.Fatal("fact lost after restart; index was not rebuilt from memory.md")
	}
}

func TestMarkdownProviderReturnsWholeFile(t *testing.T) {
	dir := t.TempDir()
	cfg := memoryTestConfig(dir)
	cfg.RAG.Provider = "markdown"
	retriever, md := chooseRetriever(cfg, nil, providers.NewManager(filepath.Join(dir, "providers")))
	if err := md.Remember(context.Background(), "喜欢喝美式"); err != nil {
		t.Fatal(err)
	}
	if !searchFinds(t, retriever, "任何问题", "喜欢喝美式") {
		t.Fatal("markdown provider did not return file content")
	}
}

func searchFinds(t *testing.T, r memory.Retriever, q, want string) bool {
	t.Helper()
	chunks, err := r.Search(context.Background(), q, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chunks {
		if strings.Contains(c.Content, want) {
			return true
		}
	}
	return false
}
