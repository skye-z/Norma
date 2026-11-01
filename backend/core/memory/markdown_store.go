// Package memory 记忆服务
package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

// 默认的个人记忆文件大小上限（字节）
const DefaultMarkdownMaxBytes = 16 * 1024

var (
	ErrMarkdownTooLarge    = errors.New("markdown memory exceeds size limit")
	ErrMarkdownBadEncoding = errors.New("markdown memory is not valid utf-8")
)

// 以单个 UTF-8 Markdown 文件保存个人资料与偏好，用户可直接查看和编辑
type MarkdownStore struct {
	mu       sync.Mutex
	path     string
	maxBytes int
}

// 创建 Markdown 记忆存储
func NewMarkdownStore(path string, maxBytes int) *MarkdownStore {
	if maxBytes <= 0 {
		maxBytes = DefaultMarkdownMaxBytes
	}
	return &MarkdownStore{path: path, maxBytes: maxBytes}
}

// 读取当前正文；文件不存在时返回空串
func (s *MarkdownStore) Read() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked()
}

func (s *MarkdownStore) readLocked() (string, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !utf8.Valid(b) {
		return "", ErrMarkdownBadEncoding
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n"), nil
}

// 覆盖写入；校验编码与大小，先写临时文件再改名，失败时保留原文件
func (s *MarkdownStore) Write(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(content)
}

func (s *MarkdownStore) writeLocked(content string) error {
	if !utf8.ValidString(content) {
		return ErrMarkdownBadEncoding
	}
	if len(content) > s.maxBytes {
		return fmt.Errorf("%w: %d > %d bytes", ErrMarkdownTooLarge, len(content), s.maxBytes)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".memory-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// 追加一条事实，作为列表项写在文件末尾；与已有条目完全相同时不重复写入
func (s *MarkdownStore) Append(fact string) error {
	fact = strings.TrimSpace(strings.ReplaceAll(fact, "\n", " "))
	if fact == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.readLocked()
	if err != nil {
		return err
	}
	line := "- " + fact
	for _, l := range strings.Split(cur, "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	if cur != "" && !strings.HasSuffix(cur, "\n") {
		cur += "\n"
	}
	return s.writeLocked(cur + line + "\n")
}

// 按行内容删除条目，返回删除的行数
func (s *MarkdownStore) Remove(substr string) (int, error) {
	substr = strings.TrimSpace(substr)
	if substr == "" {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.readLocked()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(cur, "\n")
	kept := lines[:0:0]
	removed := 0
	for _, l := range lines {
		if strings.Contains(l, substr) {
			removed++
			continue
		}
		kept = append(kept, l)
	}
	if removed == 0 {
		return 0, nil
	}
	return removed, s.writeLocked(strings.Join(kept, "\n"))
}

// 把 Markdown 正文整体作为一个检索块返回，不按问题排序，每轮对话前重新读取
type MarkdownRetriever struct{ Store *MarkdownStore }

// 实现 Retriever 接口
func (r MarkdownRetriever) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
	_ = query
	_ = topK
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := r.Store.Read()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		return nil, nil
	}
	return []Chunk{{DocID: filepath.Base(r.Store.path), ChunkID: "all", Score: 1, Content: body}}, nil
}

// 从口语指令中提取要记住的内容，如“记住我喜欢喝美式”
func ParseRememberCommand(text string) (string, bool) {
	t := strings.TrimSpace(text)
	for _, p := range []string{"请记住", "记住", "帮我记住", "记一下"} {
		if idx := strings.Index(t, p); idx >= 0 && idx <= 12 {
			rest := strings.TrimLeft(t[idx+len(p):], "，,：: ")
			rest = strings.TrimRight(rest, "。.!！")
			if strings.TrimSpace(rest) != "" {
				return rest, true
			}
		}
	}
	return "", false
}
