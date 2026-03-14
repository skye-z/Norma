package bus

import (
	"fmt"
	"sync"
	"time"
)

// 运行时事件
type Event struct {
	Name      string `json:"name"`                 // 事件名称
	SessionID string `json:"session_id,omitempty"` // 会话ID
	TraceID   string `json:"trace_id,omitempty"`   // 追踪ID
	Payload   any    `json:"payload,omitempty"`    // 载荷
	TsUnixMs  int64  `json:"ts_unix_ms"`           // 时间戳（毫秒）
}

// 事件处理函数
type Handler func(Event)

// subscription 订阅记录
type subscription struct {
	id string  // 订阅ID
	h  Handler // 处理函数
}

// 轻量级进程内发布-订阅总线
type Bus struct {
	mu        sync.RWMutex
	handlers  map[string][]subscription // 事件名 -> 订阅者列表
	wildcards []subscription            // 通配符订阅者
	seq       uint64                    // 订阅序号
}

// 创建事件总线
func New() *Bus {
	return &Bus{handlers: make(map[string][]subscription)}
}

// 发布事件
func (b *Bus) Publish(evt Event) {
	if evt.TsUnixMs == 0 {
		evt.TsUnixMs = time.Now().UnixMilli()
	}

	b.mu.RLock()
	subs := append([]subscription(nil), b.handlers[evt.Name]...)
	wildcards := append([]subscription(nil), b.wildcards...)
	b.mu.RUnlock()

	for _, sub := range subs {
		sub.h(evt)
	}
	for _, sub := range wildcards {
		sub.h(evt)
	}
}

// 订阅事件，返回取消函数
func (b *Bus) Subscribe(name string, h Handler) func() {
	b.mu.Lock()
	b.seq++
	id := fmt.Sprintf("%s#%d", name, b.seq)
	b.handlers[name] = append(b.handlers[name], subscription{id: id, h: h})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.handlers[name]
		for i := range list {
			if list[i].id == id {
				b.handlers[name] = append(list[:i], list[i+1:]...)
				if len(b.handlers[name]) == 0 {
					delete(b.handlers, name)
				}
				return
			}
		}
	}
}

// 订阅所有事件
func (b *Bus) SubscribeAll(h Handler) func() {
	b.mu.Lock()
	b.seq++
	id := fmt.Sprintf("all#%d", b.seq)
	b.wildcards = append(b.wildcards, subscription{id: id, h: h})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i := range b.wildcards {
			if b.wildcards[i].id == id {
				b.wildcards = append(b.wildcards[:i], b.wildcards[i+1:]...)
				return
			}
		}
	}
}
