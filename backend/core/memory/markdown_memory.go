// Package memory 记忆服务
package memory

import (
	"context"
	"strings"
	"sync"
)

// 记忆条目在全文与向量索引中使用的文档路径
const markdownDocPath = "memory.md"

// 索引写入接口，全文存储与向量存储都实现它
type chunkIndexer interface {
	Upsert(path string, chunks []string) error
}

// Markdown 记忆服务：文件是唯一事实来源，条目同步到全文与向量索引用于检索
type MarkdownMemory struct {
	store   *MarkdownStore
	indexes []chunkIndexer
	mu      sync.Mutex
}

// 创建 Markdown 记忆服务，indexes 为需要保持同步的索引，可为空
func NewMarkdownMemory(store *MarkdownStore, indexes ...chunkIndexer) *MarkdownMemory {
	var live []chunkIndexer
	for _, ix := range indexes {
		if ix != nil {
			live = append(live, ix)
		}
	}
	return &MarkdownMemory{store: store, indexes: live}
}

// 解析 Markdown 中的列表项，每项是一条独立事实
func ParseMarkdownEntries(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") {
			if e := strings.TrimSpace(l[2:]); e != "" {
				out = append(out, e)
			}
		}
	}
	return out
}

// 按文件当前内容重建索引；文件为空时清空对应条目
func (m *MarkdownMemory) Sync() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncLocked()
}

func (m *MarkdownMemory) syncLocked() error {
	body, err := m.store.Read()
	if err != nil {
		return err
	}
	entries := ParseMarkdownEntries(body)
	for _, ix := range m.indexes {
		if err := ix.Upsert(markdownDocPath, entries); err != nil {
			return err
		}
	}
	return nil
}

// 记住一条事实：先写文件，再同步索引。文件写入失败时不改动索引
func (m *MarkdownMemory) Remember(ctx context.Context, fact string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.Append(fact); err != nil {
		return err
	}
	return m.syncLocked()
}
