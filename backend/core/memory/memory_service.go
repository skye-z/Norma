// Package memory 记忆服务
package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// 旧版单文件记忆在索引里使用的路径；迁移后需要清掉，否则会留下过期条目
const legacyDocPath = "memory.md"

// 索引写入接口，全文存储与向量存储都实现它
type chunkIndexer interface {
	Upsert(path string, chunks []string) error
}

// MemoryService 在 MemoryBank 之上提供：显式“记住”的路由、索引同步、供后台整理使用的批量操作。
// 文件是唯一事实来源，索引随时可以重建
type MemoryService struct {
	bank    *MemoryBank
	indexes []chunkIndexer
	mu      sync.Mutex
}

func NewMemoryService(bank *MemoryBank, indexes ...chunkIndexer) *MemoryService {
	var live []chunkIndexer
	for _, ix := range indexes {
		if ix != nil {
			live = append(live, ix)
		}
	}
	return &MemoryService{bank: bank, indexes: live}
}

func (s *MemoryService) Bank() *MemoryBank { return s.bank }

// Profile 返回要注入提示词的用户画像
func (s *MemoryService) Profile() string { return s.bank.Profile() }

// 常见的“关于用户本人”的说法，用来把显式“记住……”路由到画像；其余进长期记忆
var profileFactRe = regexp.MustCompile(`^(我(叫|是|姓|的名字|的生日|喜欢|不喜欢|讨厌|爱|习惯|平时|通常|住在|来自|今年|在.{1,8}(工作|上班|读书|上学))|叫我|称呼我|用户)`)

func classifyFact(fact string) Target {
	if profileFactRe.MatchString(strings.TrimSpace(fact)) {
		return TargetUser
	}
	return TargetMemory
}

// 用户口述的“我叫 Skye”在画像里统一写成第三人称，和后台整理写入的条目风格一致
func toThirdPerson(fact string) string {
	f := strings.TrimSpace(fact)
	switch {
	case strings.HasPrefix(f, "叫我"):
		return "用户希望被称为" + strings.TrimSpace(strings.TrimPrefix(f, "叫我"))
	case strings.HasPrefix(f, "称呼我"):
		return "用户希望被称为" + strings.TrimSpace(strings.TrimPrefix(f, "称呼我"))
	case strings.HasPrefix(f, "我的"):
		return "用户的" + strings.TrimPrefix(f, "我的")
	case strings.HasPrefix(f, "我"):
		return "用户" + strings.TrimPrefix(f, "我")
	}
	return f
}

// Remember 处理用户明确说出的“记住……”：路由到画像或长期记忆，写入后同步索引。
// 超出预算返回 *MemoryFullError，内容不安全返回 ErrInvalidEntry，由调用方告诉用户
func (s *MemoryService) Remember(ctx context.Context, fact string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target := classifyFact(fact)
	fact = toThirdPerson(fact)
	if _, err := s.bank.Add(target, fact); err != nil {
		return err
	}
	return s.Sync()
}

// Op 是后台整理产出的一次记忆操作
type Op struct {
	Target  Target `json:"target"`
	Action  string `json:"action"` // add | patch | remove
	Content string `json:"content"`
	OldText string `json:"old_text"`
}

// OpResult 记录一次操作的结果，Err 为空表示已生效（add 遇到重复时 Applied 为 false 但没有错误）
type OpResult struct {
	Op      Op
	Applied bool
	Err     error
}

// Apply 依次执行操作；单条失败不影响其余，最后统一同步一次索引
func (s *MemoryService) Apply(ops []Op) []OpResult {
	res := make([]OpResult, 0, len(ops))
	changed := false
	for _, op := range ops {
		r := OpResult{Op: op}
		switch strings.ToLower(strings.TrimSpace(op.Action)) {
		case "add":
			r.Applied, r.Err = s.bank.Add(op.Target, op.Content)
		case "patch":
			r.Err = s.bank.Patch(op.Target, op.OldText, op.Content)
			r.Applied = r.Err == nil
		case "remove":
			r.Err = s.bank.Remove(op.Target, firstNonBlank(op.OldText, op.Content))
			r.Applied = r.Err == nil
		default:
			r.Err = errors.New("unknown action " + op.Action)
		}
		if r.Applied {
			changed = true
		}
		res = append(res, r)
	}
	if changed {
		_ = s.Sync()
	}
	return res
}

func firstNonBlank(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Sync 按文件当前内容重建索引：MEMORY.md 与每日笔记各用自己的相对路径作为文档路径。
// 画像每轮整体注入提示词，不进入检索索引，避免同一条信息出现两次
func (s *MemoryService) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	long, err := s.bank.Entries(TargetMemory)
	if err != nil {
		return err
	}
	docs := map[string][]string{"MEMORY.md": long}
	for _, rel := range s.bank.DailyFiles() {
		entries, err := s.bank.ReadDailyEntries(rel)
		if err != nil {
			return err
		}
		docs[rel] = entries
	}
	for _, ix := range s.indexes {
		_ = ix.Upsert(legacyDocPath, nil) // 清掉旧版单文件时代的过期条目
		for path, entries := range docs {
			if err := ix.Upsert(path, entries); err != nil {
				return err
			}
		}
	}
	return nil
}

// MigrateLegacy 把旧版单文件记忆（每行一条的 Markdown 列表）并入 MEMORY.md，并把原文件改名保留。
// 目标文件已有内容时不动，避免覆盖用户后来写的内容
func (s *MemoryService) MigrateLegacy(legacyPath string) (int, error) {
	legacyPath = strings.TrimSpace(legacyPath)
	if legacyPath == "" {
		return 0, nil
	}
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if cur, _ := s.bank.Entries(TargetMemory); len(cur) > 0 {
		return 0, nil
	}
	n := 0
	for _, e := range entriesOf(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")) {
		target := classifyFact(e)
		e = toThirdPerson(e)
		if ok, err := s.bank.Add(target, e); err == nil && ok {
			n++
		}
	}
	if err := os.Rename(legacyPath, legacyPath+".migrated"); err != nil {
		return n, err
	}
	return n, nil
}

// 路径是否属于每日笔记，并取出日期
var dailyPathRe = regexp.MustCompile(`(?:^|/)daily/(\d{4}-\d{2}-\d{2})\.md$`)

func isDailyPath(p string) bool { return dailyPathRe.MatchString(filepath.ToSlash(p)) }
