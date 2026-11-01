package memory

import (
	"context"
	"testing"
	"time"
)

type fixedChunks []Chunk

func (f fixedChunks) Search(ctx context.Context, q string, k int) ([]Chunk, error) {
	return append([]Chunk(nil), f...), nil
}

func TestOldDailyNotesLoseRankButCuratedMemoryDoesNot(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	inner := fixedChunks{
		{DocID: "daily/2026-09-03.md", Score: 1.0, Content: "两个月前的事"}, // 60 天前：衰减到 25%
		{DocID: "MEMORY.md", Score: 0.5, Content: "长期事实"},
		{DocID: "daily/2026-11-01.md", Score: 0.4, Content: "昨天的事"},
	}
	d := &DecayingRetriever{Inner: inner, HalfLifeDays: 30, Now: func() time.Time { return now }}
	got, err := d.Search(context.Background(), "q", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("decay must never drop results, got %d", len(got))
	}
	if got[0].DocID != "MEMORY.md" || got[len(got)-1].DocID != "daily/2026-09-03.md" {
		t.Fatalf("order = %v %v %v", got[0].DocID, got[1].DocID, got[2].DocID)
	}
	if s := got[2].Score; s < 0.24 || s > 0.26 {
		t.Fatalf("60-day-old note score = %.3f, want ~0.25", s)
	}
}

func TestDecayLeavesSameDayAndUndatedFilesUntouched(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	d := &DecayingRetriever{HalfLifeDays: 30, Now: func() time.Time { return now }}
	for _, id := range []string{"daily/2026-11-02.md", "daily/2026-12-01.md", "MEMORY.md", "notes/x.md", "daily/bad-date.md"} {
		if f := d.factor(id, now); f != 1 {
			t.Errorf("%s factor = %v, want 1", id, f)
		}
	}
}

func TestDecayTruncatesToTopK(t *testing.T) {
	d := NewDecayingRetriever(fixedChunks{{DocID: "MEMORY.md", Score: 3}, {DocID: "MEMORY.md", Score: 2}, {DocID: "MEMORY.md", Score: 1}})
	got, _ := d.Search(context.Background(), "q", 2)
	if len(got) != 2 || got[0].Score != 3 {
		t.Fatalf("got %+v", got)
	}
}
