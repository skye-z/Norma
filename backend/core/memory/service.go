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

const (
	KindFact       = "fact"
	KindPreference = "preference"
	KindEvent      = "event"
	KindSummary    = "summary"
)

type AddInput struct {
	Kind       string  `json:"kind"`
	Content    string  `json:"content"`
	Importance int     `json:"importance"`
	MemoryType string  `json:"memory_type"`
	Source     string  `json:"source"`
	Evidence   string  `json:"evidence"`
	Decay      float64 `json:"decay"`
	Summary    bool    `json:"summary"`
}

type UpdateInput struct {
	Kind         string  `json:"kind"`
	Content      string  `json:"content"`
	Importance   int     `json:"importance"`
	MemoryType   string  `json:"memory_type"`
	Source       string  `json:"source"`
	Evidence     string  `json:"evidence"`
	Decay        float64 `json:"decay"`
	Disabled     bool    `json:"disabled"`
	ConflictNote string  `json:"conflict_note"`
	Summary      bool    `json:"summary"`
}

type SummaryResult struct {
	Summary      *model.Memory `json:"summary,omitempty"`
	ConflictWith []int64       `json:"conflict_with,omitempty"`
}

// Service handles typed long-term memory read/write.
type Service struct {
	engine *xorm.Engine
}

func NewService(engine *xorm.Engine) *Service {
	return &Service{engine: engine}
}

func (s *Service) Add(ctx context.Context, kind, content string, importance int) (*model.Memory, error) {
	return s.AddDetailed(ctx, AddInput{Kind: kind, Content: content, Importance: importance, MemoryType: kind})
}

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
		like := "%" + q + "%"
		err := builder.And("content LIKE ? OR evidence LIKE ?", like, like).Desc("importance", "last_access", "updated_at").Limit(limit).Find(&rows)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now()
	for i := range rows {
		rows[i].LastAccess = now
		_, _ = s.engine.ID(rows[i].ID).Cols("last_access").Update(&rows[i])
	}
	return rows, nil
}

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

func (s *Service) Delete(ctx context.Context, id int64) error {
	_ = ctx
	_, err := s.engine.ID(id).Delete(&model.Memory{})
	return err
}

func (s *Service) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	_, err := s.engine.ID(id).Cols("disabled").Update(&model.Memory{Disabled: disabled})
	return err
}

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
		k := normalizeKind(r.Kind) + "::" + normalizedMemoryKey(r.Content)
		cur, ok := byKey[k]
		if !ok {
			byKey[k] = kept{ID: r.ID, Importance: r.Importance, UpdatedAt: r.UpdatedAt}
			continue
		}
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

func (s *Service) SummarizeTurn(ctx context.Context, sessionID string, turns []Turn) (*SummaryResult, error) {
	_ = sessionID
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

func (s *Service) detectConflicts(summary string) ([]int64, error) {
	rows, err := s.Search(context.Background(), summary, 20)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	summary = normalizedMemoryKey(summary)
	for _, row := range rows {
		if row.ID == 0 || normalizedMemoryKey(row.Content) == summary {
			continue
		}
		if overlapRatio(summary, normalizedMemoryKey(row.Content)) < 0.2 {
			continue
		}
		ids = append(ids, row.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindFact, KindPreference, KindEvent, KindSummary:
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return KindFact
	}
}

func normalizedMemoryKey(content string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(content))), " ")
}

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

func clampImportance(v int) int {
	if v < 0 {
		return 0
	}
	if v > 10 {
		return 10
	}
	return v
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}

func firstPositiveOr(v int, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}
