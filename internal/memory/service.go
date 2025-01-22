package memory

import (
	"context"
	"strings"
	"time"

	"xorm.io/xorm"

	"norma/internal/model"
)

const (
	KindFact       = "fact"
	KindPreference = "preference"
	KindEvent      = "event"
)

// Service handles typed long-term memory read/write.
type Service struct {
	engine *xorm.Engine
}

func NewService(engine *xorm.Engine) *Service {
	return &Service{engine: engine}
}

func (s *Service) Add(ctx context.Context, kind, content string, importance int) (*model.Memory, error) {
	_ = ctx
	kind = normalizeKind(kind)
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	if importance < 0 {
		importance = 0
	}
	if importance > 10 {
		importance = 10
	}

	m := &model.Memory{
		Kind:       kind,
		Content:    content,
		Importance: importance,
		LastAccess: time.Now(),
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
	if q == "" {
		err := sess.Desc("importance", "last_access", "updated_at").Limit(limit).Find(&rows)
		if err != nil {
			return nil, err
		}
	} else {
		like := "%" + q + "%"
		err := sess.Where("content LIKE ?", like).Desc("importance", "last_access", "updated_at").Limit(limit).Find(&rows)
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

func (s *Service) Delete(ctx context.Context, id int64) error {
	_ = ctx
	_, err := s.engine.ID(id).Delete(&model.Memory{})
	return err
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindFact, KindPreference, KindEvent:
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return KindFact
	}
}
