package runtime

import (
	"sync"
	"sync/atomic"

	"norma/internal/index"
)

// QualityController controls adaptive degradation switches.
type QualityController struct {
	autoDegrade bool
	thresholdMs int64

	mu                     sync.RWMutex
	degradedTop            int
	degradedEFSearch       int
	degradedRetrieverExtra int
	degradedRetrieverFuse  int
	degradedAudioSize      int
	degradedLLMModel       string
	degradedTTSVoice       string
	degraded               atomic.Bool
}

func NewQualityController(autoDegrade bool, thresholdMs int, degradedTop int) *QualityController {
	if thresholdMs <= 0 {
		thresholdMs = 1200
	}
	if degradedTop <= 0 {
		degradedTop = 4
	}
	return &QualityController{
		autoDegrade:            autoDegrade,
		thresholdMs:            int64(thresholdMs),
		degradedTop:            degradedTop,
		degradedEFSearch:       24,
		degradedRetrieverExtra: 1,
		degradedRetrieverFuse:  2,
		degradedAudioSize:      4096,
	}
}

func (q *QualityController) SetDegradedAudioChunkSize(size int) {
	if size <= 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.degradedAudioSize = size
}

func (q *QualityController) SetDegradedEFSearch(ef int) {
	if ef <= 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.degradedEFSearch = ef
}

func (q *QualityController) SetDegradedRetrieverFactors(extra, fuse int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if extra > 0 {
		q.degradedRetrieverExtra = extra
	}
	if fuse > 0 {
		q.degradedRetrieverFuse = fuse
	}
}

func (q *QualityController) SetDegradedLLMModel(model string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.degradedLLMModel = model
}

func (q *QualityController) SetDegradedTTSVoice(voice string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.degradedTTSVoice = voice
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
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.degradedTop
}

func (q *QualityController) SearchTuning(defaultTopK int) index.SearchTuning {
	if !q.autoDegrade || !q.degraded.Load() {
		return index.DefaultSearchTuning(defaultTopK)
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	return index.SearchTuning{
		TopK:                 q.degradedTop,
		VectorEFSearch:       q.degradedEFSearch,
		RetrieverExtraFactor: q.degradedRetrieverExtra,
		RetrieverFuseFactor:  q.degradedRetrieverFuse,
		MinVectorCandidates:  4,
		MinFusionCandidates:  8,
	}
}

func (q *QualityController) AudioChunkSize(defaultSize int) int {
	if !q.autoDegrade || !q.degraded.Load() {
		return defaultSize
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.degradedAudioSize
}

func (q *QualityController) LLMModel(defaultModel string) string {
	if !q.autoDegrade || !q.degraded.Load() {
		return defaultModel
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.degradedLLMModel != "" {
		return q.degradedLLMModel
	}
	return defaultModel
}

func (q *QualityController) TTSVoice(defaultVoice string) string {
	if !q.autoDegrade || !q.degraded.Load() {
		return defaultVoice
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.degradedTTSVoice != "" {
		return q.degradedTTSVoice
	}
	return defaultVoice
}

func (q *QualityController) IsDegraded() bool { return q.degraded.Load() }
