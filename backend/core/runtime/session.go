package runtime

import "context"

// Session 携带每个请求的取消和追踪边界
// 用于在整个请求生命周期中传递上下文和取消信号
type Session struct {
	SessionID string             // 会话唯一标识符
	TraceID   string             // 追踪唯一标识符，用于日志和调试
	Ctx       context.Context    // 上下文对象，用于传递请求级别的取消信号
	Cancel    context.CancelFunc // 取消函数，用于提前终止请求
}

// NewSession 创建一个新的Session实例
// sessionID: 会话唯一标识符
// traceID: 追踪唯一标识符
func NewSession(sessionID, traceID string) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		SessionID: sessionID,
		TraceID:   traceID,
		Ctx:       ctx,
		Cancel:    cancel,
	}
}
