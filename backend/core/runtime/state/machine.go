package state

import (
	"sync"
	"time"
)

// State 表示运行时全局状态
type State string

// 状态常量定义
const (
	Idle        State = "Idle"        // 空闲状态：初始状态，无活跃会话
	Listening   State = "Listening"   // 倾听状态：正在接收音频输入
	Thinking    State = "Thinking"    // 思考状态：正在处理 LLM 请求
	Speaking    State = "Speaking"    // 说话状态：正在播放 TTS 音频
	Interrupted State = "Interrupted" // 中断状态：会话被外部打断
	Error       State = "Error"       // 错误状态：发生错误需要处理
)

// Transition 状态转换记录，用于审计追踪
type Transition struct {
	From     State // 转换前状态
	To       State // 转换后状态
	TsUnixMs int64 // 转换时间戳（毫秒）
	Legal    bool  // 转换是否合法
}

// 历史记录数量限制
const historyLimit = 256

// Machine 线程安全的状态持有者
// 负责状态管理和状态转换审计
type Machine struct {
	mu      sync.RWMutex
	state   State        // 当前状态
	history []Transition // 状态转换历史
}

// NewMachine 创建新的状态机，初始状态为 Idle
func NewMachine() *Machine {
	return &Machine{state: Idle}
}

// Get 获取当前状态（读锁）
func (m *Machine) Get() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Set 设置新状态，执行状态转换并记录审计日志
// 如果转换非法，仍会执行转换但记录 Legal=false
func (m *Machine) Set(next State) {
	m.mu.Lock()
	defer m.mu.Unlock()

	from := m.state
	// 检查状态转换是否合法
	legal := isLegalTransition(from, next)
	m.appendHistory(Transition{
		From:     from,
		To:       next,
		TsUnixMs: time.Now().UnixMilli(),
		Legal:    legal,
	})
	m.state = next
}

// GetHistory 获取状态转换历史
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
		// 从空闲状态只能转到倾听
		return to == Listening
	case Listening:
		// 从倾听状态可以转到思考、说话、错误或中断
		return to == Thinking || to == Speaking || to == Error || to == Interrupted || to == Idle
	case Thinking:
		// 从思考状态可以转到说话、错误或中断
		return to == Speaking || to == Error || to == Interrupted || to == Idle
	case Speaking:
		// 从说话状态可以转到倾听（被打断）、错误或中断
		return to == Listening || to == Error || to == Interrupted || to == Idle
	case Interrupted:
		// 从中断状态只能转到倾听
		return to == Listening
	case Error:
		// 从错误状态可以转到倾听或空闲
		return to == Listening || to == Idle
	}
	return false
}

// appendHistory 追加转换历史，超过限制时移除最老的记录
func (m *Machine) appendHistory(t Transition) {
	m.history = append(m.history, t)
	if len(m.history) > historyLimit {
		m.history = m.history[len(m.history)-historyLimit:]
	}
}
