package runtime

import (
	"sync"

	"norma/backend/core/runtime/bus"
)

// DiagnosticSnapshot 暴露开发面板所需的最新运行时状态
type DiagnosticSnapshot struct {
	State      string      `json:"state"`       // 当前运行时状态
	QueueDepth int         `json:"queue_depth"` // 队列深度
	LastEvent  string      `json:"last_event"`  // 最后接收到的事件名称
	Events     []bus.Event `json:"events"`      // 最近的事件列表
	Sessions   []string    `json:"sessions"`    // 活跃的会话ID列表
}

// Inspector 记录最近的运行时活动，用于调试和问题排查
type Inspector struct {
	mu         sync.RWMutex
	maxEvents  int                 // 最大记录的事件数量
	state      string              // 当前运行时状态
	queueDepth int                 // 当前队列深度
	lastEvent  string              // 最后接收到的事件名称
	events     []bus.Event         // 最近的事件列表
	sessions   map[string]struct{} // 活跃会话ID集合
}

// NewInspector 创建一个新的Inspector实例
// maxEvents: 最大记录的事件数量，默认64
func NewInspector(maxEvents int) *Inspector {
	if maxEvents <= 0 {
		maxEvents = 64
	}
	return &Inspector{maxEvents: maxEvents, sessions: make(map[string]struct{})}
}

// Bind 将Inspector绑定到事件总线，用于订阅所有事件
// 返回一个取消订阅的函数
func (i *Inspector) Bind(b *bus.Bus) func() {
	if b == nil {
		return func() {}
	}
	return b.SubscribeAll(func(evt bus.Event) {
		i.Record(evt)
	})
}

// Record 记录一个事件到Inspector
// 将事件添加到事件列表，并更新会话追踪
func (i *Inspector) Record(evt bus.Event) {
	i.mu.Lock()
	defer i.mu.Unlock()
	// 记录会话ID用于追踪
	if evt.SessionID != "" {
		i.sessions[evt.SessionID] = struct{}{}
	}
	// 更新最后事件名称
	i.lastEvent = evt.Name
	// 添加事件到列表
	i.events = append(i.events, evt)
	// 保持事件列表不超过最大数量
	if len(i.events) > i.maxEvents {
		i.events = append([]bus.Event(nil), i.events[len(i.events)-i.maxEvents:]...)
	}
}

// SetState 设置当前运行时状态
func (i *Inspector) SetState(state string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.state = state
}

// SetQueueDepth 设置当前队列深度
func (i *Inspector) SetQueueDepth(depth int) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.queueDepth = depth
}

// Snapshot 获取当前运行时状态的快照
// 返回包含所有运行时诊断信息的DiagnosticSnapshot
func (i *Inspector) Snapshot() DiagnosticSnapshot {
	i.mu.RLock()
	defer i.mu.RUnlock()
	// 复制事件列表以避免并发问题
	events := append([]bus.Event(nil), i.events...)
	// 将会话集合转换为切片
	sessions := make([]string, 0, len(i.sessions))
	for sessionID := range i.sessions {
		sessions = append(sessions, sessionID)
	}
	return DiagnosticSnapshot{
		State:      i.state,
		QueueDepth: i.queueDepth,
		LastEvent:  i.lastEvent,
		Events:     events,
		Sessions:   sessions,
	}
}
