// Package config 延迟追踪和性能指标
package config

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// marks 单个会话的关键时间点
type marks struct {
	asrFinal      time.Time // ASR识别完成时间
	llmFirstToken time.Time // LLM生成首个token的时间
	ttsFirstChunk time.Time // TTS生成首个音频块的时间
}

// 单次请求的延迟数据
type Latency struct {
	ASRToLLMFirstTokenMs int64 `json:"asr_to_llm_first_token_ms"` // ASR完成到LLM首个token的毫秒数
	ASRToTTSFirstChunkMs int64 `json:"asr_to_tts_first_chunk_ms"` // ASR完成到TTS首个音频块的毫秒数
}

// 事件计数器
type EventMetric struct {
	Name      string `json:"name"`       // 事件名称
	Count     int64  `json:"count"`      // 事件发生次数
	LastValue int64  `json:"last_value"` // 最近一次的值
}

// 延迟追踪器快照
type Snapshot struct {
	Counters     []EventMetric `json:"counters"`      // 所有事件计数器
	Recent       []Latency     `json:"recent"`        // 最近50次延迟记录
	Alerts       []string      `json:"alerts"`        // 当前告警信息
	ActiveTraces int           `json:"active_traces"` // 正在进行的追踪数量
}

// 延迟追踪器
type Tracker struct {
	mu       sync.Mutex
	data     map[string]*marks       // sessionID|traceID -> 时间标记
	counters map[string]*EventMetric // 事件名称 -> 计数器
	recent   []Latency               // 最近延迟记录
	alerts   []string                // 告警信息
}

// 创建新的延迟追踪器实例
func NewTracker() *Tracker {
	return &Tracker{data: make(map[string]*marks), counters: make(map[string]*EventMetric)}
}

// key 生成会话和追踪的唯一键名
func key(sessionID, traceID string) string { return fmt.Sprintf("%s|%s", sessionID, traceID) }

// 标记ASR完成时间
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
	t.bumpLocked("asr.final", 0)
}

// 标记LLM首个token时间
func (t *Tracker) MarkLLMFirstToken(sessionID, traceID string, at time.Time) (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(sessionID, traceID)
	m := t.data[k]
	if m == nil || m.asrFinal.IsZero() || !m.llmFirstToken.IsZero() {
		return 0, false
	}
	m.llmFirstToken = at
	ms := at.Sub(m.asrFinal).Milliseconds()
	t.bumpLocked("llm.first_token", ms)
	return ms, true
}

// 标记TTS首个音频块时间
func (t *Tracker) MarkTTSFirstChunk(sessionID, traceID string, at time.Time) (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(sessionID, traceID)
	m := t.data[k]
	if m == nil || m.asrFinal.IsZero() || !m.ttsFirstChunk.IsZero() {
		return 0, false
	}
	m.ttsFirstChunk = at
	ms := at.Sub(m.asrFinal).Milliseconds()
	t.bumpLocked("tts.first_chunk", ms)
	return ms, true
}

// 记录任意事件数值
func (t *Tracker) RecordValue(name string, value int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bumpLocked(name, value)
}

// 添加告警信息
func (t *Tracker) AddAlert(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.alerts) >= 20 {
		t.alerts = append([]string(nil), t.alerts[len(t.alerts)-19:]...)
	}
	t.alerts = append(t.alerts, msg)
}

// 获取延迟快照并清除追踪数据
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
	t.recent = append(t.recent, out)
	if len(t.recent) > 50 {
		t.recent = append([]Latency(nil), t.recent[len(t.recent)-50:]...)
	}
	t.bumpLocked("trace.completed", 0)
	return out, true
}

// 获取追踪器完整快照
func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	counters := make([]EventMetric, 0, len(t.counters))
	for _, metric := range t.counters {
		counters = append(counters, *metric)
	}
	sort.Slice(counters, func(i, j int) bool { return counters[i].Name < counters[j].Name })
	recent := append([]Latency(nil), t.recent...)
	alerts := append([]string(nil), t.alerts...)
	return Snapshot{Counters: counters, Recent: recent, Alerts: alerts, ActiveTraces: len(t.data)}
}

// bumpLocked 增加事件计数
func (t *Tracker) bumpLocked(name string, value int64) {
	m := t.counters[name]
	if m == nil {
		m = &EventMetric{Name: name}
		t.counters[name] = m
	}
	m.Count++
	m.LastValue = value
}
