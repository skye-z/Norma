package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"xorm.io/xorm"

	"norma/backend/core/model"
)

// 记忆类型常量定义
const (
	KindFact       = "fact"       // 事实性记忆
	KindPreference = "preference" // 用户偏好
	KindEvent      = "event"      // 事件记录
	KindSummary    = "summary"    // 对话摘要
)

// AddInput 添加记忆的输入参数结构
type AddInput struct {
	Kind       string  `json:"kind"`        // 记忆类型
	Content    string  `json:"content"`     // 记忆内容
	Importance int     `json:"importance"`  // 重要性等级 (0-10)
	MemoryType string  `json:"memory_type"` // 记忆子类型
	Source     string  `json:"source"`      // 来源标识
	Evidence   string  `json:"evidence"`    // 证据/上下文
	Decay      float64 `json:"decay"`       // 衰减系数
	Summary    bool    `json:"summary"`     // 是否为摘要标记
}

// UpdateInput 更新记忆的输入参数结构
type UpdateInput struct {
	Kind         string  `json:"kind"`          // 记忆类型
	Content      string  `json:"content"`       // 记忆内容
	Importance   int     `json:"importance"`    // 重要性等级
	MemoryType   string  `json:"memory_type"`   // 记忆子类型
	Source       string  `json:"source"`        // 来源标识
	Evidence     string  `json:"evidence"`      // 证据/上下文
	Decay        float64 `json:"decay"`         // 衰减系数
	Disabled     bool    `json:"disabled"`      // 是否禁用
	ConflictNote string  `json:"conflict_note"` // 冲突说明
	Summary      bool    `json:"summary"`       // 是否为摘要标记
}

// SummaryResult 摘要操作的结果结构
type SummaryResult struct {
	Summary      *model.Memory `json:"summary,omitempty"`       // 生成的摘要记忆
	ConflictWith []int64       `json:"conflict_with,omitempty"` // 冲突的记忆ID列表
}

// Service 处理长期记忆的读写操作
// 使用 xorm 作为数据库访问层，支持多种记忆类型的存储和检索
type Service struct {
	engine *xorm.Engine
}

// NewService 创建记忆服务实例
func NewService(engine *xorm.Engine) *Service {
	return &Service{engine: engine}
}

// Add 添加记忆的简便方法
// 接受记忆类型、内容和重要性等级，内部调用 AddDetailed 方法
func (s *Service) Add(ctx context.Context, kind, content string, importance int) (*model.Memory, error) {
	return s.AddDetailed(ctx, AddInput{Kind: kind, Content: content, Importance: importance, MemoryType: kind})
}

// AddDetailed 添加记忆的详细方法
// 处理输入验证、格式标准化，并持久化到数据库
func (s *Service) AddDetailed(ctx context.Context, input AddInput) (*model.Memory, error) {
	_ = ctx
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, nil
	}
	importance := clampImportance(input.Importance)
	memoryType := normalizeKind(firstNonEmpty(input.MemoryType, input.Kind))
	if input.Decay < 0 {
		input.Decay = 0
	}
	m := &model.Memory{
		Kind:       normalizeKind(input.Kind),
		Content:    content,
		Importance: importance,
		LastAccess: time.Now(),
		Decay:      input.Decay,
		MemoryType: memoryType,
		Source:     strings.TrimSpace(input.Source),
		Evidence:   strings.TrimSpace(input.Evidence),
		Summary:    input.Summary,
	}
	_, err := s.engine.Insert(m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Search 搜索记忆
// 支持按关键词搜索或获取全部记忆，按重要性、最后访问时间和更新时间排序
func (s *Service) Search(ctx context.Context, query string, limit int) ([]model.Memory, error) {
	_ = ctx
	if limit <= 0 {
		limit = 10
	}

	sess := s.engine.NewSession()
	defer sess.Close()

	rows := make([]model.Memory, 0, limit)
	q := strings.TrimSpace(query)
	builder := sess.Where("disabled = ?", false)
	if q == "" {
		err := builder.Desc("importance", "last_access", "updated_at").Limit(limit).Find(&rows)
		if err != nil {
			return nil, err
		}
	} else {
		// 模糊匹配 content 和 evidence 字段
		like := "%" + q + "%"
		err := builder.And("content LIKE ? OR evidence LIKE ?", like, like).Desc("importance", "last_access", "updated_at").Limit(limit).Find(&rows)
		if err != nil {
			return nil, err
		}
	}

	// 更新最后访问时间
	now := time.Now()
	for i := range rows {
		rows[i].LastAccess = now
		_, _ = s.engine.ID(rows[i].ID).Cols("last_access").Update(&rows[i])
	}
	return rows, nil
}

// List 列出记忆
// 支持是否包含已禁用的记忆，按重要性、更新时间排序
func (s *Service) List(ctx context.Context, includeDisabled bool, limit int) ([]model.Memory, error) {
	_ = ctx
	if limit <= 0 {
		limit = 50
	}
	rows := make([]model.Memory, 0, limit)
	sess := s.engine.NewSession()
	defer sess.Close()
	if !includeDisabled {
		sess = sess.Where("disabled = ?", false)
	}
	if err := sess.Desc("importance", "updated_at", "id").Limit(limit).Find(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// Update 更新指定ID的记忆
// 使用输入字段更新记忆属性，保留原值作为默认值
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*model.Memory, error) {
	_ = ctx
	m := new(model.Memory)
	has, err := s.engine.ID(id).Get(m)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("memory not found")
	}
	m.Kind = normalizeKind(firstNonEmpty(input.Kind, m.Kind))
	m.Content = strings.TrimSpace(firstNonEmpty(input.Content, m.Content))
	m.Importance = clampImportance(firstPositiveOr(input.Importance, m.Importance))
	m.MemoryType = normalizeKind(firstNonEmpty(input.MemoryType, m.MemoryType))
	m.Source = strings.TrimSpace(firstNonEmpty(input.Source, m.Source))
	m.Evidence = strings.TrimSpace(firstNonEmpty(input.Evidence, m.Evidence))
	if input.Decay >= 0 {
		m.Decay = input.Decay
	}
	m.Disabled = input.Disabled
	m.ConflictNote = strings.TrimSpace(input.ConflictNote)
	m.Summary = input.Summary
	m.LastAccess = time.Now()
	_, err = s.engine.ID(id).AllCols().Update(m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Delete 删除指定ID的记忆
func (s *Service) Delete(ctx context.Context, id int64) error {
	_ = ctx
	_, err := s.engine.ID(id).Delete(&model.Memory{})
	return err
}

// SetDisabled 设置记忆的禁用状态
func (s *Service) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	_, err := s.engine.ID(id).Cols("disabled").Update(&model.Memory{Disabled: disabled})
	return err
}

// Compact 记忆压缩/去重
// 根据内容相似性和重要性保留最新最重要的记忆，删除重复或低优先级的记录
func (s *Service) Compact(ctx context.Context) (int64, error) {
	_ = ctx
	rows := make([]model.Memory, 0, 1024)
	if err := s.engine.Find(&rows); err != nil {
		return 0, err
	}

	type kept struct {
		ID         int64
		Importance int
		UpdatedAt  time.Time
	}
	byKey := map[string]kept{}
	for _, r := range rows {
		// 标准化 kind + content 作为去重的 key
		k := normalizeKind(r.Kind) + "::" + normalizedMemoryKey(r.Content)
		cur, ok := byKey[k]
		if !ok {
			byKey[k] = kept{ID: r.ID, Importance: r.Importance, UpdatedAt: r.UpdatedAt}
			continue
		}
		// 保留重要性更高或更新时间更晚的记录
		if r.Importance > cur.Importance || (r.Importance == cur.Importance && r.UpdatedAt.After(cur.UpdatedAt)) {
			byKey[k] = kept{ID: r.ID, Importance: r.Importance, UpdatedAt: r.UpdatedAt}
		}
	}

	keep := map[int64]struct{}{}
	for _, k := range byKey {
		keep[k.ID] = struct{}{}
	}

	var deleted int64
	for _, r := range rows {
		if _, ok := keep[r.ID]; ok {
			continue
		}
		affected, err := s.engine.ID(r.ID).Delete(&model.Memory{})
		if err != nil {
			return deleted, err
		}
		deleted += affected
	}

	return deleted, nil
}

// ApplyDecay 应用记忆衰减
// 根据距离上次访问的时间长度和衰减系数，降低记忆的重要性等级
func (s *Service) ApplyDecay(ctx context.Context) (int64, error) {
	_ = ctx
	rows := make([]model.Memory, 0, 1024)
	if err := s.engine.Find(&rows); err != nil {
		return 0, err
	}
	now := time.Now()
	var updated int64
	for _, row := range rows {
		if row.Decay <= 0 || row.Disabled {
			continue
		}
		hours := now.Sub(row.LastAccess).Hours()
		if hours <= 0 {
			continue
		}
		next := clampImportance(row.Importance - int(hours*row.Decay))
		if next == row.Importance {
			continue
		}
		row.Importance = next
		affected, err := s.engine.ID(row.ID).Cols("importance").Update(&row)
		if err != nil {
			return updated, err
		}
		updated += affected
	}
	return updated, nil
}

// SummarizeTurn 将对话轮次摘要为记忆
// 将多轮对话压缩为单条摘要记忆，并检测与现有记忆的潜在冲突
func (s *Service) SummarizeTurn(ctx context.Context, sessionID string, turns []Turn) (*SummaryResult, error) {
	_ = sessionID
	// 限制 token 数量，避免过长
	trimmed := TrimToTokenBudget(turns, 192)
	if len(trimmed) == 0 {
		return &SummaryResult{}, nil
	}
	parts := make([]string, 0, len(trimmed))
	for _, turn := range trimmed {
		if strings.TrimSpace(turn.Content) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(turn.Role)+": "+strings.TrimSpace(turn.Content))
	}
	summaryText := strings.Join(parts, " | ")
	if summaryText == "" {
		return &SummaryResult{}, nil
	}
	summary, err := s.AddDetailed(ctx, AddInput{
		Kind:       KindSummary,
		Content:    summaryText,
		Importance: 7,
		MemoryType: KindSummary,
		Source:     "conversation",
		Evidence:   summaryText,
		Summary:    true,
	})
	if err != nil {
		return nil, err
	}
	// 检测冲突
	conflicts, err := s.detectConflicts(summaryText)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 && summary != nil {
		note := "potential conflict with existing memories"
		summary.ConflictNote = note
		_, _ = s.engine.ID(summary.ID).Cols("conflict_note").Update(&model.Memory{ConflictNote: note})
	}
	return &SummaryResult{Summary: summary, ConflictWith: conflicts}, nil
}

// GovernanceRun 执行记忆治理任务
// 包括去重压缩和重要性衰减，返回处理的记录数量
func (s *Service) GovernanceRun(ctx context.Context) (map[string]int64, error) {
	deleted, err := s.Compact(ctx)
	if err != nil {
		return nil, err
	}
	decayed, err := s.ApplyDecay(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]int64{"dedup_deleted": deleted, "decayed": decayed}, nil
}

// detectConflicts 检测与现有记忆的潜在冲突
// 通过计算新摘要与已有记忆的内容重叠度来判断
func (s *Service) detectConflicts(summary string) ([]int64, error) {
	rows, err := s.Search(context.Background(), summary, 20)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	summary = normalizedMemoryKey(summary)
	for _, row := range rows {
		// 跳过自身和完全相同的内容
		if row.ID == 0 || normalizedMemoryKey(row.Content) == summary {
			continue
		}
		// 重叠度小于 20% 视为不冲突
		if overlapRatio(summary, normalizedMemoryKey(row.Content)) < 0.2 {
			continue
		}
		ids = append(ids, row.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// normalizeKind 标准化记忆类型
// 将输入转换为小写，验证是否为有效的记忆类型
func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindFact, KindPreference, KindEvent, KindSummary:
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return KindFact
	}
}

// normalizedMemoryKey 生成标准化的记忆键
// 移除标点、转小写、空格分隔，用于去重和冲突检测
func normalizedMemoryKey(content string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(content))), " ")
}

// overlapRatio 计算两个字符串的内容重叠度
// 用于判断新记忆是否与已有记忆存在冲突
func overlapRatio(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	as := map[string]struct{}{}
	for _, part := range strings.Fields(a) {
		as[part] = struct{}{}
	}
	if len(as) == 0 {
		return 0
	}
	matched := 0
	seen := map[string]struct{}{}
	for _, part := range strings.Fields(b) {
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		if _, ok := as[part]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(as))
}

// clampImportance 限制重要性等级在有效范围内
// 确保重要性值在 0-10 之间
func clampImportance(v int) int {
	if v < 0 {
		return 0
	}
	if v > 10 {
		return 10
	}
	return v
}

// firstNonEmpty 返回第一个非空字符串
// 用于在多个可选值中选择有效值
func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}

// firstPositiveOr 返回第一个正数，否则使用默认值
func firstPositiveOr(v int, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}
