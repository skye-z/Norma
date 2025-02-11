package index

import "context"

// Task represents an indexing request.
type Task struct {
    Path string
    Kind string
}

// Queue defines async indexing queue behavior.
type Queue interface {
    Enqueue(ctx context.Context, t Task) error
    Dequeue(ctx context.Context) (Task, error)
}

// MemoryQueue is a minimal in-memory queue for development.
type MemoryQueue struct {
    ch chan Task
}

func NewMemoryQueue(size int) *MemoryQueue {
    if size <= 0 {
        size = 64
    }
    return &MemoryQueue{ch: make(chan Task, size)}
}

func (q *MemoryQueue) Enqueue(ctx context.Context, t Task) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case q.ch <- t:
        return nil
    }
}

func (q *MemoryQueue) Dequeue(ctx context.Context) (Task, error) {
    select {
    case <-ctx.Done():
        return Task{}, ctx.Err()
    case t := <-q.ch:
        return t, nil
    }
}
