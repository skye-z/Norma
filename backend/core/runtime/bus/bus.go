package bus

import (
	"fmt"
	"sync"
	"time"
)

// Event 运行时事件结构体
type Event struct {
	Name      string `json:"name"`                 // 事件名称
	SessionID string `json:"session_id,omitempty"` // 会话 ID
	TraceID   string `json:"trace_id,omitempty"`   // 追踪 ID，用于串联完整调用链
	Payload   any    `json:"payload,omitempty"`    // 事件载荷
	TsUnixMs  int64  `json:"ts_unix_ms"`           // 事件时间戳（毫秒）
}

// Handler 事件处理函数类型
type Handler func(Event)

// subscription 订阅记录结构
type subscription struct {
	id string  // 订阅唯一标识
	h  Handler // 处理函数
}

// Bus 轻量级进程内发布-订阅总线
// 用于组件间的松耦合通信，支持同步事件分发
type Bus struct {
	mu        sync.RWMutex
	handlers  map[string][]subscription // 按事件名称存储订阅者
	wildcards []subscription            // 通配符订阅者（接收所有事件）
	seq       uint64                    // 订阅序号，用于生成唯一 ID
}

// New 创建新的事件总线
func New() *Bus {
	return &Bus{handlers: make(map[string][]subscription)}
}

// Publish 发布事件到总线
// 会触发所有对应事件名称的订阅者以及通配符订阅者
func (b *Bus) Publish(evt Event) {
	// 自动填充时间戳
	if evt.TsUnixMs == 0 {
		evt.TsUnixMs = time.Now().UnixMilli()
	}

	// 复制订阅列表，避免在持有锁时调用处理函数
	b.mu.RLock()
	subs := append([]subscription(nil), b.handlers[evt.Name]...)
	wildcards := append([]subscription(nil), b.wildcards...)
	b.mu.RUnlock()

	// 通知所有订阅者
	for _, sub := range subs {
		sub.h(evt)
	}
	// 通知通配符订阅者
	for _, sub := range wildcards {
		sub.h(evt)
	}
}

// Subscribe 订阅指定事件名称的事件
// 返回取消订阅函数
func (b *Bus) Subscribe(name string, h Handler) func() {
	b.mu.Lock()
	b.seq++
	id := fmt.Sprintf("%s#%d", name, b.seq)
	b.handlers[name] = append(b.handlers[name], subscription{id: id, h: h})
	b.mu.Unlock()

	// 返回取消订阅函数
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

// SubscribeAll 订阅所有事件（通配符订阅）
// 返回取消订阅函数
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
