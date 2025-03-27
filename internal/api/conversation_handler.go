package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"norma/internal/audio"
	"norma/internal/config"
	"norma/internal/runtime"
	"norma/internal/runtime/bus"
)

type ConversationHandler struct {
	bus        *bus.Bus
	inspector  *runtime.Inspector
	configPath string

	mu     sync.RWMutex
	active bool
	sid    string
	tid    string
	cancel context.CancelFunc
}

type conversationReq struct {
	SessionID string `json:"session_id"`
	TraceID   string `json:"trace_id"`
	Reason    string `json:"reason"`
}

type conversationState struct {
	Active    bool   `json:"active"`
	SessionID string `json:"session_id"`
	TraceID   string `json:"trace_id"`
	State     string `json:"state,omitempty"`
}

var conversationSeq uint64

func NewConversationHandler(configPath string, b *bus.Bus, inspector *runtime.Inspector) *ConversationHandler {
	return &ConversationHandler{configPath: configPath, bus: b, inspector: inspector}
}

func (h *ConversationHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/conversation/mic/start", h.handleMicStart)
	mux.HandleFunc("/conversation/mic/stop", h.handleMicStop)
	mux.HandleFunc("/conversation/mic/interrupt", h.handleMicInterrupt)
	mux.HandleFunc("/conversation/mic/state", h.handleMicState)
	mux.HandleFunc("/conversation/mic/test", h.handleMicTest)
}

func (h *ConversationHandler) handleMicStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req conversationReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	sid, tid := req.SessionID, req.TraceID
	if strings.TrimSpace(sid) == "" {
		sid = h.newID("mic")
	}
	if strings.TrimSpace(tid) == "" {
		tid = h.newID("trace")
	}

	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.active = true
	h.sid = sid
	h.tid = tid
	h.cancel = cancel
	h.mu.Unlock()

	go h.runMicPipeline(ctx, sid, tid)
	writeJSON(w, http.StatusOK, h.snapshot())
}

func (h *ConversationHandler) handleMicStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req conversationReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	sid, tid := h.resolveIDs(req.SessionID, req.TraceID)
	h.bus.Publish(bus.Event{Name: "audio.vad.stop", SessionID: sid, TraceID: tid, Payload: map[string]any{"source": "conversation.mic"}})

	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.active = false
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, h.snapshot())
}

func (h *ConversationHandler) handleMicInterrupt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req conversationReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	sid, tid := h.resolveIDs(req.SessionID, req.TraceID)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "mic_interrupt"
	}
	h.bus.Publish(bus.Event{Name: "interrupt", SessionID: sid, TraceID: tid, Payload: reason})

	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.active = false
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, h.snapshot())
}

func (h *ConversationHandler) handleMicState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.snapshot())
}

func (h *ConversationHandler) handleMicTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load config failed")
		return
	}
	micOK := strings.TrimSpace(cfg.Audio.MicDeviceID) != ""
	probe := audio.NewEnergyVAD(0.02)
	frames := audio.DefaultDemoFrames()
	detected := 0
	for _, frame := range frames {
		isSpeech, _ := probe.Detect(frame)
		if isSpeech {
			detected += 1
		}
	}
	ok := micOK && detected > 0
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": ok,
		"mic": map[string]any{
			"device_id":     cfg.Audio.MicDeviceID,
			"sample_rate":   cfg.Audio.MicSampleRate,
			"channel_count": cfg.Audio.MicChannelCount,
			"speech_frames": detected,
		},
		"summary": map[bool]string{true: "麦克风配置可用，VAD 探针通过", false: "麦克风配置或 VAD 探针未通过"}[ok],
	})
}

func (h *ConversationHandler) snapshot() conversationState {
	h.mu.RLock()
	state := conversationState{Active: h.active, SessionID: h.sid, TraceID: h.tid}
	h.mu.RUnlock()
	if h.inspector != nil {
		state.State = h.inspector.Snapshot().State
	}
	return state
}

func (h *ConversationHandler) resolveIDs(sessionID, traceID string) (string, string) {
	h.mu.RLock()
	sid := h.sid
	tid := h.tid
	h.mu.RUnlock()
	if strings.TrimSpace(sessionID) != "" {
		sid = sessionID
	}
	if strings.TrimSpace(traceID) != "" {
		tid = traceID
	}
	if sid == "" {
		sid = h.newID("mic")
	}
	if tid == "" {
		tid = h.newID("trace")
	}
	return sid, tid
}

func (h *ConversationHandler) newID(prefix string) string {
	seq := atomic.AddUint64(&conversationSeq, 1)
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), seq)
}

func (h *ConversationHandler) runMicPipeline(ctx context.Context, sessionID, traceID string) {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "config_load_failed", "message": err.Error()}})
		return
	}

	pipe := audio.Pipeline{
		VAD:           audio.NewEnergyVAD(0.02),
		ASR:           h.buildASR(cfg),
		WakeMode:      cfg.Audio.WakeMode,
		SilenceFrames: cfg.Audio.EndpointSilenceFrames,
	}
	if cfg.Audio.AECEnabled || cfg.Audio.NSEnabled {
		scene := strings.TrimSpace(cfg.Audio.Scene)
		if scene == "" {
			scene = "speaker"
		}
		pipe.Preprocessors = []audio.Preprocessor{audio.NewMockAECNS(scene)}
	}

	source := audio.NewMockFrameSource(audio.DefaultDemoFrames(), 40*time.Millisecond)
	err = pipe.Run(
		ctx,
		source,
		func() {
			h.bus.Publish(bus.Event{Name: "audio.vad.start", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"source": "conversation.mic.backend"}})
		},
		func() {
			h.bus.Publish(bus.Event{Name: "audio.vad.stop", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"source": "conversation.mic.backend"}})
		},
		func(partial string) {
			h.bus.Publish(bus.Event{Name: "asr.partial", SessionID: sessionID, TraceID: traceID, Payload: partial})
		},
		func(final string) {
			h.bus.Publish(bus.Event{Name: "asr.final", SessionID: sessionID, TraceID: traceID, Payload: final})
		},
	)

	h.mu.Lock()
	if h.sid == sessionID {
		h.active = false
		h.cancel = nil
	}
	h.mu.Unlock()

	if err != nil && err != context.Canceled {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "mic_pipeline_failed", "message": err.Error()}})
	}
}

func (h *ConversationHandler) buildASR(cfg *config.Config) audio.ASR {
	provider := strings.ToLower(strings.TrimSpace(cfg.ASR.Provider))
	if provider == "http" && strings.TrimSpace(cfg.ASR.Endpoint) != "" {
		asr := audio.NewHTTPASR(cfg.ASR.Endpoint, time.Duration(cfg.ASR.RequestTimeoutMs)*time.Millisecond)
		asr.StreamEndpoint = strings.TrimSpace(cfg.ASR.StreamEndpoint)
		return asr
	}
	return audio.MockASR{}
}
