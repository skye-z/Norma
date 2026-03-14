// Package memory 记忆服务
package memory

import "context"

// 索引任务
type Task struct {
	Path string
	Kind string
}

// 异步索引队列接口
type Queue interface {
	Enqueue(ctx context.Context, t Task) error
	Dequeue(ctx context.Context) (Task, error)
}

// 内存队列（开发用）
type MemoryQueue struct {
	ch chan Task
}

// 创建内存队列
func NewMemoryQueue(size int) *MemoryQueue {
	if size <= 0 {
		size = 64
	}
	return &MemoryQueue{ch: make(chan Task, size)}
}

// 入队
func (q *MemoryQueue) Enqueue(ctx context.Context, t Task) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ch <- t:
		return nil
	}
}

// 出队
func (q *MemoryQueue) Dequeue(ctx context.Context) (Task, error) {
	select {
	case <-ctx.Done():
		return Task{}, ctx.Err()
	case t := <-q.ch:
		return t, nil
	}
}
