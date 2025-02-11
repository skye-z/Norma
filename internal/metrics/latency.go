package metrics

import (
    "fmt"
    "sync"
    "time"
)

type marks struct {
    asrFinal      time.Time
    llmFirstToken time.Time
    ttsFirstChunk time.Time
}

type Latency struct {
    ASRToLLMFirstTokenMs int64
    ASRToTTSFirstChunkMs int64
}

// Tracker records per-session latency marks.
type Tracker struct {
    mu   sync.Mutex
    data map[string]*marks
}

func NewTracker() *Tracker {
    return &Tracker{data: make(map[string]*marks)}
}

func key(sessionID, traceID string) string {
    return fmt.Sprintf("%s|%s", sessionID, traceID)
}

func (t *Tracker) MarkASRFinal(sessionID, traceID string, at time.Time) {
    t.mu.Lock()
    defer t.mu.Unlock()
    k := key(sessionID, traceID)
    m := t.data[k]
    if m == nil {
        m = &marks{}
        t.data[k] = m
    }
    m.asrFinal = at
}

func (t *Tracker) MarkLLMFirstToken(sessionID, traceID string, at time.Time) (int64, bool) {
    t.mu.Lock()
    defer t.mu.Unlock()
    k := key(sessionID, traceID)
    m := t.data[k]
    if m == nil || m.asrFinal.IsZero() || !m.llmFirstToken.IsZero() {
        return 0, false
    }
    m.llmFirstToken = at
    return at.Sub(m.asrFinal).Milliseconds(), true
}

func (t *Tracker) MarkTTSFirstChunk(sessionID, traceID string, at time.Time) (int64, bool) {
    t.mu.Lock()
    defer t.mu.Unlock()
    k := key(sessionID, traceID)
    m := t.data[k]
    if m == nil || m.asrFinal.IsZero() || !m.ttsFirstChunk.IsZero() {
        return 0, false
    }
    m.ttsFirstChunk = at
    return at.Sub(m.asrFinal).Milliseconds(), true
}

func (t *Tracker) SnapshotAndClear(sessionID, traceID string) (Latency, bool) {
    t.mu.Lock()
    defer t.mu.Unlock()
    k := key(sessionID, traceID)
    m := t.data[k]
    if m == nil || m.asrFinal.IsZero() {
        return Latency{}, false
    }
    out := Latency{}
    if !m.llmFirstToken.IsZero() {
        out.ASRToLLMFirstTokenMs = m.llmFirstToken.Sub(m.asrFinal).Milliseconds()
    }
    if !m.ttsFirstChunk.IsZero() {
        out.ASRToTTSFirstChunkMs = m.ttsFirstChunk.Sub(m.asrFinal).Milliseconds()
    }
    delete(t.data, k)
    return out, true
}
