package runtime

import "context"

// Session carries cancellation and tracing boundaries per request.
type Session struct {
    SessionID string
    TraceID   string
    Ctx       context.Context
    Cancel    context.CancelFunc
}

func NewSession(sessionID, traceID string) *Session {
    ctx, cancel := context.WithCancel(context.Background())
    return &Session{
        SessionID: sessionID,
        TraceID:   traceID,
        Ctx:       ctx,
        Cancel:    cancel,
    }
}
