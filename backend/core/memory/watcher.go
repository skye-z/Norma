package memory

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors file changes and enqueues indexing tasks.
type Watcher struct {
	watcher *fsnotify.Watcher
	queue   Queue
}

func NewWatcher(queue Queue) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{watcher: w, queue: queue}, nil
}

func (w *Watcher) Close() error {
	if w == nil || w.watcher == nil {
		return nil
	}
	return w.watcher.Close()
}

func (w *Watcher) AddRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldSkipPath(path) {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return w.watcher.Add(path)
		}
		return nil
	})
}

// EnqueueInitial enqueues all indexable files under root for upsert.
func (w *Watcher) EnqueueInitial(ctx context.Context, root string) error {
	if w.queue == nil {
		return errors.New("watcher queue is nil")
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldSkipPath(path) {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d != nil && d.IsDir() {
			return nil
		}
		if !isIndexable(path) {
			return nil
		}
		return w.queue.Enqueue(ctx, Task{Path: path, Kind: "upsert"})
	})
}

func (w *Watcher) Run(ctx context.Context) error {
	if w.queue == nil {
		return errors.New("watcher queue is nil")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(ctx, ev)
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
		}
	}
}

func (w *Watcher) handleEvent(ctx context.Context, ev fsnotify.Event) {
	if w.queue == nil || shouldSkipPath(ev.Name) {
		return
	}

	if isCreateOrWrite(ev.Op) && isIndexable(ev.Name) {
		_ = w.queue.Enqueue(ctx, Task{Path: ev.Name, Kind: "upsert"})
	}

	if ev.Op&fsnotify.Remove == fsnotify.Remove || ev.Op&fsnotify.Rename == fsnotify.Rename {
		_ = w.queue.Enqueue(ctx, Task{Path: ev.Name, Kind: "delete"})
	}

	if ev.Op&fsnotify.Create == fsnotify.Create && w.watcher != nil {
		if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
			_ = w.watcher.Add(ev.Name)
		}
	}
}

func isCreateOrWrite(op fsnotify.Op) bool {
	return op&fsnotify.Create == fsnotify.Create || op&fsnotify.Write == fsnotify.Write
}

func isIndexable(path string) bool {
	if shouldSkipPath(path) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".pdf", ".docx":
		return true
	default:
		return false
	}
}
