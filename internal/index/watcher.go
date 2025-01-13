package index

import (
	"context"
	"errors"
	"io/fs"
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
		if d.IsDir() {
			return w.watcher.Add(path)
		}
		return nil
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

			if isCreateOrWrite(ev.Op) && isIndexable(ev.Name) {
				_ = w.queue.Enqueue(ctx, Task{Path: ev.Name, Kind: "upsert"})
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				_ = w.queue.Enqueue(ctx, Task{Path: ev.Name, Kind: "delete"})
			}
			if ev.Op&fsnotify.Create == fsnotify.Create {
				// If a new directory appears, add it to watcher.
				_ = w.watcher.Add(ev.Name)
			}
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
		}
	}
}

func isCreateOrWrite(op fsnotify.Op) bool {
	return op&fsnotify.Create == fsnotify.Create || op&fsnotify.Write == fsnotify.Write
}

func isIndexable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		return true
	default:
		return false
	}
}
