package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"norma/internal/audio"
	"norma/internal/bootstrap"
	"norma/internal/config"
	"norma/internal/providers"
	"norma/internal/runtime"
	"norma/internal/runtime/bus"
)

type ConversationHandler struct {
	bus        *bus.Bus
	inspector  *runtime.Inspector
	configPath string
	providers  *providers.Manager
	streams    map[string]*audio.PushFrameSource
	sources    map[string]audio.FrameSource
	capture    map[string]string
	modes      map[string]string

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
	Mode      string `json:"mode"`
}

type conversationState struct {
	Active    bool   `json:"active"`
	SessionID string `json:"session_id"`
	TraceID   string `json:"trace_id"`
	State     string `json:"state,omitempty"`
	Capture   string `json:"capture,omitempty"`
	Mode      string `json:"mode,omitempty"`
}

var conversationSeq uint64

func NewConversationHandler(configPath string, b *bus.Bus, inspector *runtime.Inspector, providerMgr *providers.Manager) *ConversationHandler {
	return &ConversationHandler{configPath: configPath, bus: b, inspector: inspector, providers: providerMgr, streams: make(map[string]*audio.PushFrameSource), sources: make(map[string]audio.FrameSource), capture: make(map[string]string), modes: make(map[string]string)}
}

func (h *ConversationHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/conversation/mic/start", h.handleMicStart)
	mux.HandleFunc("/conversation/mic/stop", h.handleMicStop)
	mux.HandleFunc("/conversation/mic/interrupt", h.handleMicInterrupt)
	mux.HandleFunc("/conversation/mic/state", h.handleMicState)
	mux.HandleFunc("/conversation/mic/test", h.handleMicTest)
	mux.HandleFunc("/conversation/mic/frame", h.handleMicFrame)
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
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load config failed")
		return
	}

	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	if prev := h.streams[sid]; prev != nil {
		prev.Close()
		delete(h.streams, sid)
	}
	if prevSource := h.sources[sid]; prevSource != nil {
		if closable, ok := prevSource.(interface{ Close() error }); ok {
			_ = closable.Close()
		}
		delete(h.sources, sid)
	}
	delete(h.capture, sid)
	delete(h.modes, sid)
	ctx, cancel := context.WithCancel(context.Background())
	var source audio.FrameSource
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "manual"
	}
	captureMode := "native"
	if strings.EqualFold(mode, "manual") {
		stream := audio.NewPushFrameSource(64)
		h.streams[sid] = stream
		source = stream
		captureMode = "push"
	} else {
		platformSource, sourceErr := audio.NewPlatformCaptureSource(cfg.Audio.MicSampleRate, cfg.Audio.MicChannelCount)
		if sourceErr != nil {
			h.bus.Publish(bus.Event{Name: "error", SessionID: sid, TraceID: tid, Payload: map[string]string{"code": "mic_capture_unavailable", "message": sourceErr.Error()}})
			stream := audio.NewPushFrameSource(64)
			h.streams[sid] = stream
			source = stream
			captureMode = "push"
		} else {
			source = platformSource
		}
	}
	h.active = true
	h.sid = sid
	h.tid = tid
	h.cancel = cancel
	h.sources[sid] = source
	h.capture[sid] = captureMode
	h.modes[sid] = mode
	h.mu.Unlock()

	go h.runMicPipeline(ctx, sid, tid, source, mode)
	log.Printf("[conversation] mic start session=%s trace=%s mode=%s capture=%s", sid, tid, mode, captureMode)
	state := h.snapshot()
	state.Capture = captureMode
	state.Mode = mode
	writeJSON(w, http.StatusOK, state)
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
	if stream := h.streams[sid]; stream != nil {
		stream.Close()
		delete(h.streams, sid)
	}
	if src := h.sources[sid]; src != nil {
		if closable, ok := src.(interface{ Close() error }); ok {
			_ = closable.Close()
		}
		delete(h.sources, sid)
	}
	delete(h.capture, sid)
	delete(h.modes, sid)
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
	log.Printf("[conversation] mic interrupt session=%s trace=%s reason=%s capture=%s mode=%s", sid, tid, reason, h.capture[sid], h.modes[sid])

	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	if stream := h.streams[sid]; stream != nil {
		stream.Close()
		delete(h.streams, sid)
	}
	if src := h.sources[sid]; src != nil {
		if bg, ok := src.(audio.BackgroundCaptureController); ok {
			bg.EnableBackgroundCapture(sid, true)
		}
	}
	if h.capture[sid] == "push" {
		delete(h.capture, sid)
	}
	delete(h.modes, sid)
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

type micFrameReq struct {
	SessionID string    `json:"session_id"`
	TraceID   string    `json:"trace_id"`
	Frames    []float32 `json:"frames"`
	EOF       bool      `json:"eof"`
}

func (h *ConversationHandler) handleMicFrame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req micFrameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	sid, tid := h.resolveIDs(req.SessionID, req.TraceID)
	h.mu.RLock()
	stream := h.streams[sid]
	mode := h.capture[sid]
	h.mu.RUnlock()
	if mode != "push" {
		writeErr(w, http.StatusBadRequest, "conversation stream is not in push capture mode")
		return
	}
	if stream == nil {
		writeErr(w, http.StatusNotFound, "conversation stream not found")
		return
	}
	accepted := false
	if len(req.Frames) > 0 {
		accepted = stream.Push(req.Frames)
	}
	if req.EOF {
		stream.Close()
		h.mu.Lock()
		delete(h.streams, sid)
		h.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "accepted": accepted, "session_id": sid, "trace_id": tid})
}

func (h *ConversationHandler) snapshot() conversationState {
	h.mu.RLock()
	state := conversationState{Active: h.active, SessionID: h.sid, TraceID: h.tid}
	if h.sid != "" {
		state.Capture = h.capture[h.sid]
		state.Mode = h.modes[h.sid]
	}
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

func (h *ConversationHandler) runMicPipeline(ctx context.Context, sessionID, traceID string, source audio.FrameSource, mode string) {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "config_load_failed", "message": err.Error()}})
		return
	}

	providerMgr := h.providers
	if providerMgr == nil {
		providerMgr = providers.NewManager(cfg.Providers.Root)
		_ = providerMgr.EnsureLayout()
	}
	pipeMode := ""
	if strings.EqualFold(strings.TrimSpace(mode), "manual") {
		pipeMode = audio.WakeModeVADOnly
	}
	pipe := bootstrap.BuildVoicePipelineForMode(cfg, providerMgr, pipeMode)
	wakeStatus := bootstrap.ResolveWakeConfig(cfg, providerMgr)
	if wakeStatus.Enabled && !wakeStatus.Ready {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "wake_word_unavailable", "message": wakeStatus.Reason}})
	}

	if closable, ok := any(source).(interface{ Close() error }); ok {
		defer func() {
			h.mu.RLock()
			_, stillRegistered := h.sources[sessionID]
			h.mu.RUnlock()
			if stillRegistered {
				_ = closable.Close()
			}
		}()
	}

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
		delete(h.streams, sessionID)
		delete(h.sources, sessionID)
		delete(h.capture, sessionID)
		delete(h.modes, sessionID)
	}
	h.mu.Unlock()

	if err != nil && err != context.Canceled {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "mic_pipeline_failed", "message": err.Error()}})
	}
}
