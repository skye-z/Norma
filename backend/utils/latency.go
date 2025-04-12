// Package config 提供延迟追踪和性能指标收集功能
// 用于监控ASR->LLM->TTS语音链路的延迟性能
package config

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// marks 记录单个会话的关键时间点
// 用于计算语音处理链路中各阶段的延迟
type marks struct {
	asrFinal      time.Time // ASR识别完成时间
	llmFirstToken time.Time // LLM生成首个token的时间
	ttsFirstChunk time.Time // TTS生成首个音频块的时间
}

// Latency 结构体表示单次请求的延迟数据
// 记录从ASR完成到LLM首个token、以及到TTS首个音频块的延迟
type Latency struct {
	ASRToLLMFirstTokenMs int64 `json:"asr_to_llm_first_token_ms"` // ASR完成到LLM首个token的毫秒数
	ASRToTTSFirstChunkMs int64 `json:"asr_to_tts_first_chunk_ms"` // ASR完成到TTS首个音频块的毫秒数
}

// EventMetric 结构体用于记录事件计数器
// 跟踪各类事件的发生次数和最新值
type EventMetric struct {
	Name      string `json:"name"`       // 事件名称
	Count     int64  `json:"count"`      // 事件发生次数
	LastValue int64  `json:"last_value"` // 最近一次的值
}

// Snapshot 结构体提供延迟追踪器的完整快照
// 用于导出当前状态供监控和调试使用
type Snapshot struct {
	Counters     []EventMetric `json:"counters"`      // 所有事件计数器
	Recent       []Latency     `json:"recent"`        // 最近50次延迟记录
	Alerts       []string      `json:"alerts"`        // 当前告警信息
	ActiveTraces int           `json:"active_traces"` // 正在进行的追踪数量
}

// Tracker 延迟追踪器，记录每个会话的各阶段时间点
// 线程安全，支持并发访问
type Tracker struct {
	mu       sync.Mutex
	data     map[string]*marks       // sessionID|traceID -> 时间标记
	counters map[string]*EventMetric // 事件名称 -> 计数器
	recent   []Latency               // 最近延迟记录
	alerts   []string                // 告警信息
}

// NewTracker 创建新的延迟追踪器实例
func NewTracker() *Tracker {
	return &Tracker{data: make(map[string]*marks), counters: make(map[string]*EventMetric)}
}

// key 生成会话和追踪的唯一键名
func key(sessionID, traceID string) string { return fmt.Sprintf("%s|%s", sessionID, traceID) }

// MarkASRFinal 标记ASR识别完成的时间点
// 当语音识别完成时调用此方法记录时间，用于后续延迟计算
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

// MarkLLMFirstToken 标记LLM生成首个token的时间点
// 返回从ASR完成到此时间点的毫秒数，以及是否成功标记
// 只有在已有ASR完成标记且尚未标记LLM首个token时才成功
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

// MarkTTSFirstChunk 标记TTS生成首个音频块的时间点
// 返回从ASR完成到此时间点的毫秒数，以及是否成功标记
// 只有在已有ASR完成标记且尚未标记TTS首个块时才成功
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

// RecordValue 记录任意事件的数值
// 用于记录各类自定义指标，如token数量、错误次数等
func (t *Tracker) RecordValue(name string, value int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bumpLocked(name, value)
}

// AddAlert 添加告警信息
// 最多保留最近20条告警，超出时移除最旧的
func (t *Tracker) AddAlert(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.alerts) >= 20 {
		t.alerts = append([]string(nil), t.alerts[len(t.alerts)-19:]...)
	}
	t.alerts = append(t.alerts, msg)
}

// SnapshotAndClear 获取指定会话的延迟快照并清除追踪数据
// 返回该会话的延迟数据以及是否存在有效数据
// 调用后清除该会话的所有追踪状态
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

// Snapshot 获取追踪器的完整快照
// 返回所有计数器、最近延迟记录、告警和活跃追踪数
// 线程安全，可随时调用
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

// bumpLocked 内部方法：增加事件计数
// 调用前必须已持有锁
func (t *Tracker) bumpLocked(name string, value int64) {
	m := t.counters[name]
	if m == nil {
		m = &EventMetric{Name: name}
		t.counters[name] = m
	}
	m.Count++
	m.LastValue = value
}
