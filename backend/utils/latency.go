package config

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type marks struct {
	asrFinal      time.Time
	llmFirstToken time.Time
	ttsFirstChunk time.Time
}

type Latency struct {
	ASRToLLMFirstTokenMs int64 `json:"asr_to_llm_first_token_ms"`
	ASRToTTSFirstChunkMs int64 `json:"asr_to_tts_first_chunk_ms"`
}

type EventMetric struct {
	Name      string `json:"name"`
	Count     int64  `json:"count"`
	LastValue int64  `json:"last_value"`
}

type Snapshot struct {
	Counters     []EventMetric `json:"counters"`
	Recent       []Latency     `json:"recent"`
	Alerts       []string      `json:"alerts"`
	ActiveTraces int           `json:"active_traces"`
}

// Tracker records per-session latency marks.
type Tracker struct {
	mu       sync.Mutex
	data     map[string]*marks
	counters map[string]*EventMetric
	recent   []Latency
	alerts   []string
}

func NewTracker() *Tracker {
	return &Tracker{data: make(map[string]*marks), counters: make(map[string]*EventMetric)}
}

func key(sessionID, traceID string) string { return fmt.Sprintf("%s|%s", sessionID, traceID) }

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

func (t *Tracker) RecordValue(name string, value int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bumpLocked(name, value)
}

func (t *Tracker) AddAlert(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.alerts) >= 20 {
		t.alerts = append([]string(nil), t.alerts[len(t.alerts)-19:]...)
	}
	t.alerts = append(t.alerts, msg)
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
	t.recent = append(t.recent, out)
	if len(t.recent) > 50 {
		t.recent = append([]Latency(nil), t.recent[len(t.recent)-50:]...)
	}
	t.bumpLocked("trace.completed", 0)
	return out, true
}

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

func (t *Tracker) bumpLocked(name string, value int64) {
	m := t.counters[name]
	if m == nil {
		m = &EventMetric{Name: name}
		t.counters[name] = m
	}
	m.Count++
	m.LastValue = value
}
