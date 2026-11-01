// Package memory 记忆服务
package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// 记忆目标：用户画像 或 长期记忆。参考 Hermes 的 USER.md / MEMORY.md 与 OpenClaw 的分层
type Target string

const (
	TargetUser   Target = "user"
	TargetMemory Target = "memory"
)

const (
	DefaultUserLimit   = 1200 // 画像每轮注入提示词，必须小
	DefaultMemoryLimit = 8000 // 长期记忆按相关度检索，不整体注入，可以大一些
	maxEntryRunes      = 300
	minEntryRunes      = 3 // 更短的内容（如“我”）说明识别被截断了，存下来只会污染记忆
)

var (
	ErrNoMatch      = errors.New("no memory entry matches")
	ErrAmbiguous    = errors.New("more than one memory entry matches")
	ErrInvalidEntry = errors.New("invalid memory entry")
)

// MemoryFullError 记忆超出字符预算。与 Hermes 一致：不悄悄丢条目，而是报错并带上当前条目，由调用方合并或删除后重试
type MemoryFullError struct {
	Target  Target
	Used    int
	Limit   int
	Over    int
	Entries []string
}

func (e *MemoryFullError) Error() string {
	return fmt.Sprintf("%s memory full: %d/%d chars, over by %d", e.Target, e.Used, e.Limit, e.Over)
}

// 条目写入前的检查结果的拒绝原因
type invalidReason string

// MemoryBank 管理 USER.md（画像）、MEMORY.md（长期记忆）与 daily/YYYY-MM-DD.md（每日笔记）。
// 文件是普通 Markdown 列表，用户可以直接查看和修改；写入先写临时文件再改名
type MemoryBank struct {
	dir         string
	userLimit   int
	memoryLimit int
	now         func() time.Time

	mu sync.Mutex
}

func NewMemoryBank(dir string, userLimit, memoryLimit int) *MemoryBank {
	if userLimit <= 0 {
		userLimit = DefaultUserLimit
	}
	if memoryLimit <= 0 {
		memoryLimit = DefaultMemoryLimit
	}
	return &MemoryBank{dir: dir, userLimit: userLimit, memoryLimit: memoryLimit, now: time.Now}
}

func (b *MemoryBank) Dir() string { return b.dir }

func (b *MemoryBank) path(t Target) string {
	if t == TargetUser {
		return filepath.Join(b.dir, "USER.md")
	}
	return filepath.Join(b.dir, "MEMORY.md")
}

func (b *MemoryBank) limit(t Target) int {
	if t == TargetUser {
		return b.userLimit
	}
	return b.memoryLimit
}

func validTarget(t Target) bool { return t == TargetUser || t == TargetMemory }

// 读取文件所有行；文件不存在返回空
func (b *MemoryBank) readLines(t Target) ([]string, error) {
	data, err := os.ReadFile(b.path(t))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, ErrMarkdownBadEncoding
	}
	text := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func isEntryLine(l string) bool {
	l = strings.TrimSpace(l)
	return strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")
}

func entryText(l string) string { return strings.TrimSpace(strings.TrimSpace(l)[2:]) }

func entriesOf(lines []string) []string {
	var out []string
	for _, l := range lines {
		if isEntryLine(l) {
			if e := entryText(l); e != "" {
				out = append(out, e)
			}
		}
	}
	return out
}

func usedChars(entries []string) int {
	n := 0
	for _, e := range entries {
		n += utf8.RuneCountInString(e)
	}
	return n
}

func (b *MemoryBank) writeLines(t Target, lines []string) error {
	if err := os.MkdirAll(b.dir, 0o755); err != nil {
		return err
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	tmp, err := os.CreateTemp(b.dir, ".memory-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, b.path(t)); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Entries 返回条目文本（不含标题、注释等非列表行）
func (b *MemoryBank) Entries(t Target) ([]string, error) {
	if !validTarget(t) {
		return nil, fmt.Errorf("%w: unknown target %q", ErrInvalidEntry, t)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	lines, err := b.readLines(t)
	if err != nil {
		return nil, err
	}
	return entriesOf(lines), nil
}

// Usage 返回已用与上限（字符数）
func (b *MemoryBank) Usage(t Target) (used, limit int, err error) {
	entries, err := b.Entries(t)
	return usedChars(entries), b.limit(t), err
}

// Profile 返回要注入提示词的用户画像，一行一条；没有内容返回空串
func (b *MemoryBank) Profile() string {
	entries, err := b.Entries(TargetUser)
	if err != nil || len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString("- " + e + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func normalizeKey(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// Add 添加一条；完全重复（忽略空白与标点）时返回 added=false；超出预算返回 *MemoryFullError
func (b *MemoryBank) Add(t Target, content string) (added bool, err error) {
	if !validTarget(t) {
		return false, fmt.Errorf("%w: unknown target %q", ErrInvalidEntry, t)
	}
	entry, err := SanitizeMemoryEntry(content)
	if err != nil {
		return false, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	lines, err := b.readLines(t)
	if err != nil {
		return false, err
	}
	entries := entriesOf(lines)
	key := normalizeKey(entry)
	for _, e := range entries {
		if normalizeKey(e) == key {
			return false, nil
		}
	}
	used := usedChars(entries)
	need := utf8.RuneCountInString(entry)
	if used+need > b.limit(t) {
		return false, &MemoryFullError{Target: t, Used: used, Limit: b.limit(t), Over: used + need - b.limit(t), Entries: entries}
	}
	return true, b.writeLines(t, append(lines, "- "+entry))
}

// 唯一匹配 old 的条目下标；找不到或多于一条分别返回 ErrNoMatch、ErrAmbiguous
func uniqueMatch(lines []string, old string) (int, error) {
	old = strings.TrimSpace(old)
	if old == "" {
		return -1, fmt.Errorf("%w: empty match text", ErrInvalidEntry)
	}
	idx, exact := -1, -1
	n := 0
	for i, l := range lines {
		if !isEntryLine(l) {
			continue
		}
		e := entryText(l)
		if e == old {
			exact = i
		}
		if strings.Contains(e, old) {
			idx = i
			n++
		}
	}
	if exact >= 0 { // 整条完全一致时优先，与 Hermes 一致
		return exact, nil
	}
	switch {
	case n == 0:
		return -1, ErrNoMatch
	case n > 1:
		return -1, ErrAmbiguous
	}
	return idx, nil
}

// Patch 只替换条目里的 old 那一段，其余内容保留。old 必须唯一定位一条，且在该条中只出现一次
func (b *MemoryBank) Patch(t Target, old, replacement string) error {
	if !validTarget(t) {
		return fmt.Errorf("%w: unknown target %q", ErrInvalidEntry, t)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	lines, err := b.readLines(t)
	if err != nil {
		return err
	}
	i, err := uniqueMatch(lines, old)
	if err != nil {
		return err
	}
	cur := entryText(lines[i])
	old = strings.TrimSpace(old)
	if strings.Count(cur, old) != 1 {
		return fmt.Errorf("%w: text appears %d times in the entry", ErrAmbiguous, strings.Count(cur, old))
	}
	updated, err := SanitizeMemoryEntry(strings.Replace(cur, old, strings.TrimSpace(replacement), 1))
	if err != nil {
		return err
	}
	next := append([]string(nil), lines...)
	next[i] = "- " + updated
	if used := usedChars(entriesOf(next)); used > b.limit(t) {
		return &MemoryFullError{Target: t, Used: used, Limit: b.limit(t), Over: used - b.limit(t), Entries: entriesOf(lines)}
	}
	return b.writeLines(t, next)
}

// Remove 删除唯一匹配 sub 的那一条；匹配多条时报错，不会误删
func (b *MemoryBank) Remove(t Target, sub string) error {
	if !validTarget(t) {
		return fmt.Errorf("%w: unknown target %q", ErrInvalidEntry, t)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	lines, err := b.readLines(t)
	if err != nil {
		return err
	}
	i, err := uniqueMatch(lines, sub)
	if err != nil {
		return err
	}
	return b.writeLines(t, append(append([]string(nil), lines[:i]...), lines[i+1:]...))
}

// DailyPath 是某一天的每日笔记路径，如 daily/2026-11-02.md
func (b *MemoryBank) DailyPath(day time.Time) string {
	return filepath.Join(b.dir, "daily", day.Format("2006-01-02")+".md")
}

// AppendDaily 往今天的每日笔记追加一条带时间的记录；每日笔记不整体注入提示词，只通过检索取用
func (b *MemoryBank) AppendDaily(text string) error {
	entry, err := SanitizeMemoryEntry(text)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	p := b.DailyPath(now)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- %s %s\n", now.Format("15:04"), entry)
	return err
}

// DailyFiles 列出所有每日笔记文件（相对 dir 的路径，如 daily/2026-11-02.md）
func (b *MemoryBank) DailyFiles() []string {
	matches, _ := filepath.Glob(filepath.Join(b.dir, "daily", "*.md"))
	var out []string
	for _, m := range matches {
		if rel, err := filepath.Rel(b.dir, m); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
	}
	return out
}

// ReadDailyEntries 读取某个每日笔记的条目文本
func (b *MemoryBank) ReadDailyEntries(rel string) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(b.dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, ErrMarkdownBadEncoding
	}
	return entriesOf(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")), nil
}

var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+|any\s+|the\s+)?(previous|prior|above|earlier)\s+(instructions?|prompts?|rules?)`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+|the\s+)?(previous|prior|above|system)`),
	regexp.MustCompile(`(?i)(system\s+prompt|developer\s+message|jailbreak)`),
	regexp.MustCompile(`(?i)^\s*(system|assistant|user)\s*[:：]`),
	regexp.MustCompile(`(?i)(\[/?inst\]|<\|[a-z_]+\|>|<<sys>>)`),
	regexp.MustCompile(`(忽略|无视|忘记|不要理会)(以上|之前|前面|上面|先前|所有|全部)?(的)?(所有|全部)?(指令|指示|提示|规则|设定|要求)`),
	regexp.MustCompile(`(系统提示词?|系统指令|越狱)`),
	regexp.MustCompile(`(从现在开始|从此刻起|现在起)[，,]?你(是|必须|要|不再)`),
	regexp.MustCompile(`(?i)(curl|wget)\s+[^|]*\|\s*(ba)?sh`),
	regexp.MustCompile(`(?i)ssh-(rsa|ed25519)\s+[A-Za-z0-9+/]{20,}`),
	// 凭据不该进记忆：它们会被注入提示词并发给云端模型
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_ -]?key|密码|口令|密钥)\s*(是|为|[:：=])\s*\S{4,}`),
}

// SanitizeMemoryEntry 在条目进入记忆前做检查并规整为单行。
// 条目会被注入提示词，而内容来自用户的语音，所以按不可信输入处理：
// 拒绝过长、含控制或不可见字符、含提示词注入或凭据特征的内容（与 Hermes 写入前的扫描一致）
func SanitizeMemoryEntry(s string) (string, error) {
	s = strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " "))
	if s == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidEntry)
	}
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("%w: not valid utf-8", ErrInvalidEntry)
	}
	if n := utf8.RuneCountInString(s); n < minEntryRunes || s == "用户" {
		return "", fmt.Errorf("%w: too short to be a fact", ErrInvalidEntry)
	}
	if n := utf8.RuneCountInString(s); n > maxEntryRunes {
		return "", fmt.Errorf("%w: too long (%d > %d chars)", ErrInvalidEntry, n, maxEntryRunes)
	}
	for _, r := range s {
		switch {
		case unicode.IsControl(r):
			return "", fmt.Errorf("%w: contains control characters", ErrInvalidEntry)
		case r >= 0x200B && r <= 0x200F, r >= 0x202A && r <= 0x202E, r >= 0x2060 && r <= 0x2064, r == 0xFEFF, r >= 0x2066 && r <= 0x2069:
			return "", fmt.Errorf("%w: contains invisible characters", ErrInvalidEntry)
		}
	}
	for _, re := range injectionPatterns {
		if re.MatchString(s) {
			return "", fmt.Errorf("%w: looks like a prompt-injection or credential pattern", ErrInvalidEntry)
		}
	}
	return s, nil
}
