package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

type captureQueue struct {
	tasks []Task
}

func (q *captureQueue) Enqueue(ctx context.Context, t Task) error {
	_ = ctx
	q.tasks = append(q.tasks, t)
	return nil
}

func (q *captureQueue) Dequeue(ctx context.Context) (Task, error) {
	_ = ctx
	if len(q.tasks) == 0 {
		return Task{}, context.Canceled
	}
	t := q.tasks[0]
	q.tasks = q.tasks[1:]
	return t, nil
}

func TestIsIndexableExtensions(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"a.txt", true},
		{"a.md", true},
		{"a.pdf", true},
		{"a.docx", true},
		{"a.png", false},
	}
	for _, c := range cases {
		if got := isIndexable(c.path); got != c.want {
			t.Fatalf("isIndexable(%q)=%v want %v", c.path, got, c.want)
		}
	}
}

func TestWatcherHandleEventRenameAsDelete(t *testing.T) {
	q := &captureQueue{}
	w := &Watcher{queue: q}
	w.handleEvent(context.Background(), fsnotify.Event{Name: "doc.md", Op: fsnotify.Rename})

	if len(q.tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(q.tasks))
	}
	if q.tasks[0].Kind != "delete" {
		t.Fatalf("expected delete task, got %q", q.tasks[0].Kind)
	}
}

func TestWatcherHandleEventCreateAsUpsert(t *testing.T) {
	q := &captureQueue{}
	w := &Watcher{queue: q}
	w.handleEvent(context.Background(), fsnotify.Event{Name: "notes.pdf", Op: fsnotify.Create})

	if len(q.tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(q.tasks))
	}
	if q.tasks[0].Kind != "upsert" || q.tasks[0].Path != "notes.pdf" {
		t.Fatalf("unexpected task: %#v", q.tasks[0])
	}
}

func TestWatcherEnqueueInitial(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name string) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mustWrite("a.md")
	mustWrite("b.pdf")
	mustWrite("c.png")

	q := &captureQueue{}
	w := &Watcher{queue: q}
	if err := w.EnqueueInitial(context.Background(), dir); err != nil {
		t.Fatalf("enqueue initial: %v", err)
	}

	if len(q.tasks) != 2 {
		t.Fatalf("expected 2 index tasks, got %d (%#v)", len(q.tasks), q.tasks)
	}
	for _, task := range q.tasks {
		if task.Kind != "upsert" {
			t.Fatalf("expected upsert, got %q", task.Kind)
		}
	}
}
