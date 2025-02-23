package runtime

import "sync/atomic"

// QualityController controls adaptive degradation switches.
type QualityController struct {
	autoDegrade       bool
	thresholdMs       int64
	degradedTop       int
	degradedAudioSize int
	degraded          atomic.Bool
}

func NewQualityController(autoDegrade bool, thresholdMs int, degradedTop int) *QualityController {
	if thresholdMs <= 0 {
		thresholdMs = 1200
	}
	if degradedTop <= 0 {
		degradedTop = 4
	}
	return &QualityController{autoDegrade: autoDegrade, thresholdMs: int64(thresholdMs), degradedTop: degradedTop, degradedAudioSize: 4096}
}

func (q *QualityController) SetDegradedAudioChunkSize(size int) {
	if size > 0 {
		q.degradedAudioSize = size
	}
}

func (q *QualityController) ObserveLLMFirstToken(ms int64) {
	if !q.autoDegrade {
		return
	}
	q.degraded.Store(ms > q.thresholdMs)
}

func (q *QualityController) ObservePressure(activeTraces int) {
	if !q.autoDegrade {
		return
	}
	if activeTraces >= 4 {
		q.degraded.Store(true)
	}
}

func (q *QualityController) RAGTopK(defaultTopK int) int {
	if !q.autoDegrade || !q.degraded.Load() {
		return defaultTopK
	}
	return q.degradedTop
}

func (q *QualityController) AudioChunkSize(defaultSize int) int {
	if !q.autoDegrade || !q.degraded.Load() {
		return defaultSize
	}
	return q.degradedAudioSize
}

func (q *QualityController) IsDegraded() bool { return q.degraded.Load() }
