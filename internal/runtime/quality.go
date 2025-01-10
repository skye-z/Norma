package runtime

import (
    "sync/atomic"
)

// QualityController controls adaptive degradation switches.
type QualityController struct {
    autoDegrade bool
    thresholdMs int64
    degradedTop int
    degraded    atomic.Bool
}

func NewQualityController(autoDegrade bool, thresholdMs int, degradedTop int) *QualityController {
    if thresholdMs <= 0 {
        thresholdMs = 1200
    }
    if degradedTop <= 0 {
        degradedTop = 4
    }
    return &QualityController{
        autoDegrade: autoDegrade,
        thresholdMs: int64(thresholdMs),
        degradedTop: degradedTop,
    }
}

func (q *QualityController) ObserveLLMFirstToken(ms int64) {
    if !q.autoDegrade {
        return
    }
    if ms > q.thresholdMs {
        q.degraded.Store(true)
        return
    }
    q.degraded.Store(false)
}

func (q *QualityController) RAGTopK(defaultTopK int) int {
    if !q.autoDegrade || !q.degraded.Load() {
        return defaultTopK
    }
    return q.degradedTop
}

func (q *QualityController) IsDegraded() bool {
    return q.degraded.Load()
}
