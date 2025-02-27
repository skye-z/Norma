package memory

import (
	"context"
	"path/filepath"
	"testing"

	"norma/internal/db"
)

func TestMemoryServiceAddSearchDelete(t *testing.T) {
	engine, err := db.NewEngine(filepath.Join(t.TempDir(), "m.db"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	svc := NewService(engine)
	m, err := svc.AddDetailed(context.Background(), AddInput{
		Kind:       KindPreference,
		Content:    "喜欢简洁回答",
		Importance: 8,
		MemoryType: KindPreference,
		Source:     "user_profile",
		Evidence:   "用户明确要求回答简洁",
		Decay:      0.1,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if m == nil || m.ID == 0 {
		t.Fatalf("expected inserted memory")
	}

	hits, err := svc.Search(context.Background(), "简洁", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected search hits")
	}
	if hits[0].Source != "user_profile" {
		t.Fatalf("expected source to be stored")
	}

	updated, err := svc.Update(context.Background(), m.ID, UpdateInput{ConflictNote: "待用户确认", Disabled: true})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.Disabled {
		t.Fatalf("expected disabled=true")
	}

	if err := svc.Delete(context.Background(), m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestMemoryCompactAndGovernance(t *testing.T) {
	engine, err := db.NewEngine(filepath.Join(t.TempDir(), "c.db"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	svc := NewService(engine)
	_, _ = svc.AddDetailed(context.Background(), AddInput{Kind: KindFact, Content: "同一事实", Importance: 1, Decay: 0.5})
	keep, _ := svc.AddDetailed(context.Background(), AddInput{Kind: KindFact, Content: "同一事实", Importance: 8, Decay: 0.5})
	_, _ = svc.AddDetailed(context.Background(), AddInput{Kind: KindEvent, Content: "事件A", Importance: 5})

	deleted, err := svc.Compact(context.Background())
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if deleted == 0 {
		t.Fatalf("expected compact to delete duplicates")
	}

	hits, err := svc.Search(context.Background(), "同一事实", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 deduped row, got %d", len(hits))
	}
	if hits[0].ID != keep.ID {
		t.Fatalf("expected higher importance row kept")
	}

	stats, err := svc.GovernanceRun(context.Background())
	if err != nil {
		t.Fatalf("governance: %v", err)
	}
	if _, ok := stats["dedup_deleted"]; !ok {
		t.Fatalf("expected governance stats")
	}
}

func TestMemorySummaryAndConflict(t *testing.T) {
	engine, err := db.NewEngine(filepath.Join(t.TempDir(), "s.db"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	svc := NewService(engine)
	_, _ = svc.AddDetailed(context.Background(), AddInput{Kind: KindFact, Content: "用户计划今天修复记忆系统", Importance: 7})
	result, err := svc.SummarizeTurn(context.Background(), "session-1", []Turn{
		{Role: "user", Content: "今天要继续修复记忆系统", Importance: 8, Pinned: true},
		{Role: "assistant", Content: "我会先补齐字段和治理任务", Importance: 7},
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if result.Summary == nil || !result.Summary.Summary {
		t.Fatalf("expected summary memory to be created")
	}
}
