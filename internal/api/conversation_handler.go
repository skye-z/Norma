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

	"github.com/gorilla/websocket"

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
	sockets    map[string]*websocket.Conn
	capture    map[string]string
	modes      map[string]string
	frameSeen  map[string]bool

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

type conversationPolicy struct {
	AutoWake      bool                       `json:"auto_wake"`
	WakeReady     bool                       `json:"wake_ready"`
	ShowMicButton bool                       `json:"show_mic_button"`
	WakeStatus    bootstrap.WakeConfigStatus `json:"wake_status"`
}

var conversationSeq uint64

var conversationWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewConversationHandler(configPath string, b *bus.Bus, inspector *runtime.Inspector, providerMgr *providers.Manager) *ConversationHandler {
	return &ConversationHandler{
		configPath: configPath,
		bus:        b,
		inspector:  inspector,
		providers:  providerMgr,
		streams:    make(map[string]*audio.PushFrameSource),
		sources:    make(map[string]audio.FrameSource),
		sockets:    make(map[string]*websocket.Conn),
		capture:    make(map[string]string),
		modes:      make(map[string]string),
		frameSeen:  make(map[string]bool),
	}
}

func (h *ConversationHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/conversation/mic/start", h.handleMicStart)
	mux.HandleFunc("/conversation/mic/stop", h.handleMicStop)
	mux.HandleFunc("/conversation/mic/interrupt", h.handleMicInterrupt)
	mux.HandleFunc("/conversation/mic/state", h.handleMicState)
	mux.HandleFunc("/conversation/mic/test", h.handleMicTest)
	mux.HandleFunc("/conversation/mic/frame", h.handleMicFrame)
	mux.HandleFunc("/conversation/mic/ws", h.handleMicWS)
	mux.HandleFunc("/conversation/mic/policy", h.handleMicPolicy)
}

func (h *ConversationHandler) handleMicPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load config failed")
		return
	}
	writeJSON(w, http.StatusOK, h.resolvePolicy(cfg))
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
	for id, stream := range h.streams {
		if stream != nil {
			stream.Close()
		}
		delete(h.streams, id)
	}
	for id, src := range h.sources {
		if closable, ok := src.(interface{ Close() error }); ok {
			_ = closable.Close()
		}
		delete(h.sources, id)
	}
	for id, conn := range h.sockets {
		if conn != nil {
			_ = conn.Close()
		}
		delete(h.sockets, id)
	}
	for id := range h.capture {
		delete(h.capture, id)
	}
	for id := range h.modes {
		delete(h.modes, id)
	}
	for id := range h.frameSeen {
		delete(h.frameSeen, id)
	}

	ctx, cancel := context.WithCancel(context.Background())
	mode := normalizeConversationMode(req.Mode)
	stream := audio.NewPushFrameSource(64)
	h.streams[sid] = stream
	h.sources[sid] = stream
	h.capture[sid] = "push"
	h.modes[sid] = mode
	h.active = true
	h.sid = sid
	h.tid = tid
	h.cancel = cancel
	h.mu.Unlock()

	go h.runMicPipeline(ctx, sid, tid, stream, mode)
	log.Printf("[conversation] mic start session=%s trace=%s mode=%s capture=push", sid, tid, mode)
	state := h.snapshot()
	state.Capture = "push"
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

	h.clearSession(sid)
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

	h.clearSession(sid)
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
			detected++
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
	accepted, statusCode, message := h.ingestMicFrame(sid, tid, req)
	if message != "" {
		writeErr(w, statusCode, message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "accepted": accepted, "session_id": sid, "trace_id": tid})
}

func (h *ConversationHandler) handleMicWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sid, tid := h.resolveIDs(r.URL.Query().Get("session_id"), r.URL.Query().Get("trace_id"))
	conn, err := conversationWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	stream := h.streams[sid]
	captureMode := h.capture[sid]
	if captureMode != "push" || stream == nil {
		h.mu.Unlock()
		_ = conn.WriteJSON(map[string]any{"ok": false, "error": "conversation stream not found"})
		_ = conn.Close()
		return
	}
	if prev := h.sockets[sid]; prev != nil && prev != conn {
		_ = prev.Close()
	}
	h.sockets[sid] = conn
	h.mu.Unlock()
	log.Printf("[conversation] mic ws connected session=%s trace=%s", sid, tid)

	defer func() {
		shouldClear := false
		h.mu.Lock()
		current := h.sockets[sid]
		if current == conn {
			delete(h.sockets, sid)
			shouldClear = h.sid == sid
		}
		h.mu.Unlock()
		_ = conn.Close()
		if shouldClear {
			h.clearSession(sid)
		}
		log.Printf("[conversation] mic ws closed session=%s trace=%s", sid, tid)
	}()

	for {
		var req micFrameReq
		if err := conn.ReadJSON(&req); err != nil {
			return
		}
		if strings.TrimSpace(req.SessionID) == "" {
			req.SessionID = sid
		}
		if strings.TrimSpace(req.TraceID) == "" {
			req.TraceID = tid
		}
		accepted, statusCode, message := h.ingestMicFrame(sid, tid, req)
		if message != "" {
			_ = conn.WriteJSON(map[string]any{"ok": false, "status": statusCode, "error": message})
			return
		}
		if req.EOF {
			_ = conn.WriteJSON(map[string]any{"ok": true, "accepted": accepted, "session_id": sid, "trace_id": tid, "eof": true})
			return
		}
	}
}

func (h *ConversationHandler) ingestMicFrame(sid, tid string, req micFrameReq) (bool, int, string) {
	h.mu.RLock()
	stream := h.streams[sid]
	captureMode := h.capture[sid]
	mode := h.modes[sid]
	h.mu.RUnlock()
	if captureMode != "push" {
		return false, http.StatusBadRequest, "conversation stream is not in push capture mode"
	}
	if stream == nil {
		return false, http.StatusNotFound, "conversation stream not found"
	}
	accepted := false
	if len(req.Frames) > 0 {
		accepted = stream.Push(req.Frames)
		if accepted {
			h.mu.Lock()
			firstFrame := !h.frameSeen[sid]
			if firstFrame {
				h.frameSeen[sid] = true
			}
			h.mu.Unlock()
			if firstFrame {
				log.Printf("[conversation] mic frame stream active session=%s trace=%s mode=%s frames=%d", sid, tid, mode, len(req.Frames))
			}
		}
	}
	if req.EOF {
		stream.Close()
		h.mu.Lock()
		delete(h.streams, sid)
		if conn := h.sockets[sid]; conn != nil {
			_ = conn.Close()
			delete(h.sockets, sid)
		}
		h.mu.Unlock()
	}
	return accepted, http.StatusOK, ""
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
	if strings.EqualFold(mode, "manual") {
		pipeMode = audio.WakeModeVADOnly
	}
	pipe := bootstrap.BuildVoicePipelineForMode(cfg, providerMgr, pipeMode)
	wakeStatus := bootstrap.ResolveWakeConfig(cfg, providerMgr)
	log.Printf("[conversation] pipeline init session=%s trace=%s mode=%s wake_enabled=%v wake_ready=%v wake_mode=%s provider=%s keyword=%s keywords=%s model=%s reason=%s", sessionID, traceID, mode, wakeStatus.Enabled, wakeStatus.Ready, wakeStatus.EffectiveMode, wakeStatus.Provider, wakeStatus.Keyword, wakeStatus.KeywordsFile, wakeStatus.ModelDir, wakeStatus.Reason)
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
			normalized := audio.PostProcessASRText(final, cfg.Audio.WakeKeyword)
			if normalized != final {
				log.Printf("[asr] postprocess raw=%q normalized=%q", final, normalized)
			}
			h.bus.Publish(bus.Event{Name: "asr.final", SessionID: sessionID, TraceID: traceID, Payload: normalized})
		},
	)

	h.mu.Lock()
	if h.sid == sessionID {
		h.active = false
		h.cancel = nil
		delete(h.streams, sessionID)
		delete(h.sources, sessionID)
		if conn := h.sockets[sessionID]; conn != nil {
			_ = conn.Close()
			delete(h.sockets, sessionID)
		}
		delete(h.capture, sessionID)
		delete(h.modes, sessionID)
		delete(h.frameSeen, sessionID)
	}
	h.mu.Unlock()

	if err != nil && err != context.Canceled {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "mic_pipeline_failed", "message": err.Error()}})
	}
}

func (h *ConversationHandler) clearSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	if stream := h.streams[sessionID]; stream != nil {
		stream.Close()
	}
	if conn := h.sockets[sessionID]; conn != nil {
		_ = conn.Close()
	}
	if src := h.sources[sessionID]; src != nil {
		if bg, ok := src.(audio.BackgroundCaptureController); ok {
			bg.EnableBackgroundCapture(sessionID, true)
		}
		if closable, ok := src.(interface{ Close() error }); ok {
			_ = closable.Close()
		}
	}
	delete(h.streams, sessionID)
	delete(h.sources, sessionID)
	delete(h.sockets, sessionID)
	delete(h.capture, sessionID)
	delete(h.modes, sessionID)
	delete(h.frameSeen, sessionID)
	if h.sid == sessionID {
		h.active = false
		h.sid = ""
		h.tid = ""
	}
}

func (h *ConversationHandler) resolvePolicy(cfg *config.Config) conversationPolicy {
	providerMgr := h.providers
	if providerMgr == nil {
		providerMgr = providers.NewManager(cfg.Providers.Root)
		_ = providerMgr.EnsureLayout()
	}
	wakeStatus := bootstrap.ResolveWakeConfig(cfg, providerMgr)
	autoWake := cfg != nil && cfg.Audio.EnableWakeWord && wakeStatus.EffectiveMode == audio.WakeModeWakeWordVAD
	return conversationPolicy{
		AutoWake:      autoWake,
		WakeReady:     wakeStatus.Ready,
		ShowMicButton: !autoWake || !wakeStatus.Ready,
		WakeStatus:    wakeStatus,
	}
}

func normalizeConversationMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "manual", "direct":
		return "manual"
	case "wake":
		return "wake"
	default:
		return "manual"
	}
}
