package state

import (
	"sync"
	"time"
)

// 运行时全局状态
type State string

// 状态常量
const (
	Idle        State = "Idle"        // 空闲
	Listening   State = "Listening"   // 倾听
	Thinking    State = "Thinking"    // 思考
	Speaking    State = "Speaking"    // 说话
	Interrupted State = "Interrupted" // 中断
	Error       State = "Error"       // 错误
)

// 状态转换记录
type Transition struct {
	From     State // 转换前
	To       State // 转换后
	TsUnixMs int64 // 时间戳（毫秒）
	Legal    bool  // 是否合法
}

const historyLimit = 256

// 线程安全的状态机
type Machine struct {
	mu      sync.RWMutex
	state   State        // 当前状态
	history []Transition // 转换历史
}

// 创建状态机
func NewMachine() *Machine {
	return &Machine{state: Idle}
}

// 获取当前状态
func (m *Machine) Get() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// 设置新状态；非法转换只记录审计、不改变状态，返回是否生效
func (m *Machine) Set(next State) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	from := m.state
	if from == next {
		return true
	}
	legal := isLegalTransition(from, next)
	m.appendHistory(Transition{
		From:     from,
		To:       next,
		TsUnixMs: time.Now().UnixMilli(),
		Legal:    legal,
	})
	if legal {
		m.state = next
	}
	return legal
}

// 获取转换历史
func (m *Machine) GetHistory() []Transition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Transition, len(m.history))
	copy(result, m.history)
	return result
}

// isLegalTransition 检查状态转换是否合法
func isLegalTransition(from, to State) bool {
	switch from {
	case Idle:
		return to == Listening || to == Thinking
	case Listening:
		return to == Thinking || to == Speaking || to == Error || to == Interrupted || to == Idle
	case Thinking:
		return to == Speaking || to == Listening || to == Error || to == Interrupted || to == Idle
	case Speaking:
		return to == Listening || to == Thinking || to == Error || to == Interrupted || to == Idle
	case Interrupted:
		return to == Listening || to == Idle
	case Error:
		return to == Listening || to == Idle
	}
	return false
}

// appendHistory 追加转换历史
func (m *Machine) appendHistory(t Transition) {
	m.history = append(m.history, t)
	if len(m.history) > historyLimit {
		m.history = m.history[len(m.history)-historyLimit:]
	}
}

// History 返回状态转换历史的副本
func (m *Machine) History() []Transition { return m.GetHistory() }
