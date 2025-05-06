// Package api 提供 HTTP API
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

	"norma/backend/core/audio"
	audioAsr "norma/backend/core/audio/asr"
	audioVad "norma/backend/core/audio/vad"
	"norma/backend/core/bootstrap"
	"norma/backend/core/providers"
	"norma/backend/core/runtime"
	"norma/backend/core/runtime/bus"
	"norma/backend/utils"
)

// ConversationHandler 对话处理器
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

// conversationReq 对话请求
type conversationReq struct {
	SessionID string `json:"session_id"`
	TraceID   string `json:"trace_id"`
	Reason    string `json:"reason"`
	Mode      string `json:"mode"`
}

// conversationState 对话状态
type conversationState struct {
	Active    bool   `json:"active"`
	SessionID string `json:"session_id"`
	TraceID   string `json:"trace_id"`
	State     string `json:"state,omitempty"`
	Capture   string `json:"capture,omitempty"`
	Mode      string `json:"mode,omitempty"`
}

// conversationPolicy 对话策略
type conversationPolicy struct {
	AutoWake      bool                       `json:"auto_wake"`
	WakeReady     bool                       `json:"wake_ready"`
	ShowMicButton bool                       `json:"show_mic_button"`
	WakeStatus    bootstrap.WakeConfigStatus `json:"wake_status"`
}

// 全局会话序列号，用于生成唯一 ID
var conversationSeq uint64

// WebSocket 升级器配置
var conversationWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 允许所有来源
}

// NewConversationHandler 创建对话处理器
func NewConversationHandler(configPath string, b *bus.Bus, inspector *runtime.Inspector, providerMgr *providers.Manager) *ConversationHandler {
	return &ConversationHandler{
		configPath: configPath,
		bus:        b,
		inspector:  inspector,
		providers:  providerMgr,
		// 初始化各 map 以避免 nil 指针
		streams:   make(map[string]*audio.PushFrameSource),
		sources:   make(map[string]audio.FrameSource),
		sockets:   make(map[string]*websocket.Conn),
		capture:   make(map[string]string),
		modes:     make(map[string]string),
		frameSeen: make(map[string]bool),
	}
}

// Register 注册路由
func (h *ConversationHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/conversation/mic/start", h.handleMicStart)         // 启动麦克风
	mux.HandleFunc("/conversation/mic/stop", h.handleMicStop)           // 停止麦克风
	mux.HandleFunc("/conversation/mic/interrupt", h.handleMicInterrupt) // 中断对话
	mux.HandleFunc("/conversation/mic/state", h.handleMicState)         // 获取状态
	mux.HandleFunc("/conversation/mic/test", h.handleMicTest)           // 测试麦克风
	mux.HandleFunc("/conversation/mic/frame", h.handleMicFrame)         // 推送音频帧
	mux.HandleFunc("/conversation/mic/ws", h.handleMicWS)               // WebSocket 连接
	mux.HandleFunc("/conversation/mic/policy", h.handleMicPolicy)       // 获取策略配置
}

// handleMicPolicy 获取对话策略
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

// handleMicStart 启动麦克风
func (h *ConversationHandler) handleMicStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req conversationReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	// 生成或使用提供的会话 ID 和追踪 ID
	sid, tid := req.SessionID, req.TraceID
	if strings.TrimSpace(sid) == "" {
		sid = h.newID("mic")
	}
	if strings.TrimSpace(tid) == "" {
		tid = h.newID("trace")
	}

	// 清理之前的会话资源
	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	// 关闭并清理所有现有的音频流
	for id, stream := range h.streams {
		if stream != nil {
			stream.Close()
		}
		delete(h.streams, id)
	}
	// 关闭并清理所有音频源
	for id, src := range h.sources {
		if closable, ok := src.(interface{ Close() error }); ok {
			_ = closable.Close()
		}
		delete(h.sources, id)
	}
	// 关闭并清理所有 WebSocket 连接
	for id, conn := range h.sockets {
		if conn != nil {
			_ = conn.Close()
		}
		delete(h.sockets, id)
	}
	// 清理捕获模式和对话模式
	for id := range h.capture {
		delete(h.capture, id)
	}
	for id := range h.modes {
		delete(h.modes, id)
	}
	for id := range h.frameSeen {
		delete(h.frameSeen, id)
	}

	// 创建新的对话上下文和音频推送源
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

	// 在后台运行麦克风处理管道
	go h.runMicPipeline(ctx, sid, tid, stream, mode)
	log.Printf("[conversation] mic start session=%s trace=%s mode=%s capture=push", sid, tid, mode)
	// 返回当前状态快照
	state := h.snapshot()
	state.Capture = "push"
	state.Mode = mode
	writeJSON(w, http.StatusOK, state)
}

// handleMicStop 停止麦克风
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
	// 发布 VAD 停止事件
	h.bus.Publish(bus.Event{Name: "audio.vad.stop", SessionID: sid, TraceID: tid, Payload: map[string]any{"source": "conversation.mic"}})

	h.clearSession(sid)
	writeJSON(w, http.StatusOK, h.snapshot())
}

// handleMicInterrupt 中断对话
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
	// 发布中断事件
	h.bus.Publish(bus.Event{Name: "interrupt", SessionID: sid, TraceID: tid, Payload: reason})
	log.Printf("[conversation] mic interrupt session=%s trace=%s reason=%s capture=%s mode=%s", sid, tid, reason, h.capture[sid], h.modes[sid])

	h.clearSession(sid)
	writeJSON(w, http.StatusOK, h.snapshot())
}

// handleMicState 获取对话状态
func (h *ConversationHandler) handleMicState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.snapshot())
}

// handleMicTest 测试麦克风
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
	// 检查麦克风设备是否配置
	micOK := strings.TrimSpace(cfg.Audio.MicDeviceID) != ""
	// 使用能量 VAD 探针测试音频帧检测
	probe := audioVad.NewEnergyVAD(0.02)
	frames := audio.DefaultDemoFrames()
	detected := 0
	for _, frame := range frames {
		isSpeech, _ := probe.Detect(frame)
		if isSpeech {
			detected++
		}
	}
	// 判断测试是否通过
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

// micFrameReq 麦克风音频帧请求
type micFrameReq struct {
	SessionID  string    `json:"session_id"`
	TraceID    string    `json:"trace_id"`
	SampleRate int       `json:"sample_rate"`
	Frames     []float32 `json:"frames"`
	EOF        bool      `json:"eof"`
}

// handleMicFrame 接收麦克风音频帧
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

// handleMicWS WebSocket 连接
func (h *ConversationHandler) handleMicWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sid, tid := h.resolveIDs(r.URL.Query().Get("session_id"), r.URL.Query().Get("trace_id"))
	// 升级 HTTP 连接为 WebSocket 连接
	conn, err := conversationWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// 检查会话是否存在
	h.mu.Lock()
	stream := h.streams[sid]
	captureMode := h.capture[sid]
	if captureMode != "push" || stream == nil {
		h.mu.Unlock()
		_ = conn.WriteJSON(map[string]any{"ok": false, "error": "conversation stream not found"})
		_ = conn.Close()
		return
	}
	// 关闭之前的连接
	if prev := h.sockets[sid]; prev != nil && prev != conn {
		_ = prev.Close()
	}
	h.sockets[sid] = conn
	h.mu.Unlock()
	log.Printf("[conversation] mic ws connected session=%s trace=%s", sid, tid)

	// 处理 WebSocket 连接生命周期
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

	// 循环读取客户端发送的音频帧
	for {
		var req micFrameReq
		if err := conn.ReadJSON(&req); err != nil {
			return
		}
		// 使用默认会话 ID 和追踪 ID
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
		// 收到 EOF 信号后关闭连接
		if req.EOF {
			_ = conn.WriteJSON(map[string]any{"ok": true, "accepted": accepted, "session_id": sid, "trace_id": tid, "eof": true})
			return
		}
	}
}

// ingestMicFrame 推送音频帧到流
func (h *ConversationHandler) ingestMicFrame(sid, tid string, req micFrameReq) (bool, int, string) {
	h.mu.RLock()
	stream := h.streams[sid]
	captureMode := h.capture[sid]
	mode := h.modes[sid]
	h.mu.RUnlock()

	// 验证捕获模式
	if captureMode != "push" {
		return false, http.StatusBadRequest, "conversation stream is not in push capture mode"
	}
	if stream == nil {
		return false, http.StatusNotFound, "conversation stream not found"
	}

	accepted := false
	if len(req.Frames) > 0 {
		// 采样率转换：将非 16kHz 的采样率转换为 16kHz
		frame := req.Frames
		if req.SampleRate > 0 && req.SampleRate != 16000 {
			frame = resampleConversationFrame(req.Frames, req.SampleRate, 16000)
		}
		accepted = stream.Push(frame)
		if accepted {
			h.mu.Lock()
			firstFrame := !h.frameSeen[sid]
			if firstFrame {
				h.frameSeen[sid] = true
			}
			h.mu.Unlock()
			// 记录首次收到音频帧
			if firstFrame {
				log.Printf("[conversation] mic frame stream active session=%s trace=%s mode=%s frames=%d", sid, tid, mode, len(req.Frames))
			}
		}
	}
	// 处理 EOF 信号，关闭流并清理资源
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

// resampleConversationFrame 音频帧重采样
func resampleConversationFrame(frame []float32, inputRate, targetRate int) []float32 {
	// 边界情况检查
	if len(frame) == 0 || inputRate <= 0 || targetRate <= 0 || inputRate == targetRate {
		return frame
	}
	// 不支持上采样
	if inputRate < targetRate {
		return frame
	}
	// 计算下采样比率
	ratio := float64(inputRate) / float64(targetRate)
	length := int(float64(len(frame)) / ratio)
	if length <= 0 {
		length = 1
	}
	// 简单的平均池化重采样
	out := make([]float32, length)
	offset := 0
	for i := 0; i < length; i++ {
		nextOffset := int(float64(i+1) * ratio)
		if nextOffset > len(frame) {
			nextOffset = len(frame)
		}
		if nextOffset <= offset {
			nextOffset = offset + 1
			if nextOffset > len(frame) {
				nextOffset = len(frame)
			}
		}
		var sum float32
		count := 0
		for j := offset; j < nextOffset; j++ {
			sum += frame[j]
			count++
		}
		if count > 0 {
			out[i] = sum / float32(count)
		} else {
			out[i] = frame[offset]
		}
		offset = nextOffset
		if offset >= len(frame) {
			offset = len(frame) - 1
		}
	}
	return out
}

// snapshot 获取状态快照
func (h *ConversationHandler) snapshot() conversationState {
	h.mu.RLock()
	state := conversationState{Active: h.active, SessionID: h.sid, TraceID: h.tid}
	if h.sid != "" {
		state.Capture = h.capture[h.sid]
		state.Mode = h.modes[h.sid]
	}
	h.mu.RUnlock()
	// 从检查器获取运行时状态
	if h.inspector != nil {
		state.State = h.inspector.Snapshot().State
	}
	return state
}

// resolveIDs 解析会话ID和追踪ID
func (h *ConversationHandler) resolveIDs(sessionID, traceID string) (string, string) {
	h.mu.RLock()
	sid := h.sid
	tid := h.tid
	h.mu.RUnlock()
	// 优先使用请求中提供的 ID
	if strings.TrimSpace(sessionID) != "" {
		sid = sessionID
	}
	if strings.TrimSpace(traceID) != "" {
		tid = traceID
	}
	// 生成默认 ID 如果仍为空
	if sid == "" {
		sid = h.newID("mic")
	}
	if tid == "" {
		tid = h.newID("trace")
	}
	return sid, tid
}

// newID 生成唯一ID
func (h *ConversationHandler) newID(prefix string) string {
	seq := atomic.AddUint64(&conversationSeq, 1)
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), seq)
}

// runMicPipeline 运行麦克风处理管道
func (h *ConversationHandler) runMicPipeline(ctx context.Context, sessionID, traceID string, source audio.FrameSource, mode string) {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "config_load_failed", "message": err.Error()}})
		return
	}

	// 初始化提供商管理器
	providerMgr := h.providers
	if providerMgr == nil {
		providerMgr = providers.NewManager(cfg.Providers.Root)
		_ = providerMgr.EnsureLayout()
	}
	// 根据模式配置管道
	pipeMode := ""
	if strings.EqualFold(mode, "manual") {
		pipeMode = audio.WakeModeVADOnly
	}
	// 构建语音处理管道
	pipe := bootstrap.BuildVoicePipelineForMode(cfg, providerMgr, pipeMode)
	wakeStatus := bootstrap.ResolveWakeConfig(cfg, providerMgr)
	log.Printf("[conversation] pipeline init session=%s trace=%s mode=%s wake_enabled=%v wake_ready=%v wake_mode=%s provider=%s keyword=%s keywords=%s model=%s reason=%s", sessionID, traceID, mode, wakeStatus.Enabled, wakeStatus.Ready, wakeStatus.EffectiveMode, wakeStatus.Provider, wakeStatus.Keyword, wakeStatus.KeywordsFile, wakeStatus.ModelDir, wakeStatus.Reason)
	// 如果启用唤醒词但未准备就绪，发布错误事件
	if wakeStatus.Enabled && !wakeStatus.Ready {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "wake_word_unavailable", "message": wakeStatus.Reason}})
	}

	// 设置资源清理回调
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

	// 运行处理管道，注册回调函数
	err = pipe.Run(
		ctx,
		source,
		// VAD 开始回调
		func() {
			h.bus.Publish(bus.Event{Name: "audio.vad.start", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"source": "conversation.mic.backend"}})
		},
		// VAD 结束回调
		func() {
			h.bus.Publish(bus.Event{Name: "audio.vad.stop", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"source": "conversation.mic.backend"}})
		},
		// ASR 部分结果回调
		func(partial string) {
			h.bus.Publish(bus.Event{Name: "asr.partial", SessionID: sessionID, TraceID: traceID, Payload: partial})
		},
		// ASR 最终结果回调
		func(final string) {
			normalized := audioAsr.PostProcessASRText(final, cfg.Audio.WakeKeyword)
			if normalized != final {
				log.Printf("[asr] postprocess raw=%q normalized=%q", final, normalized)
			}
			h.bus.Publish(bus.Event{Name: "asr.final", SessionID: sessionID, TraceID: traceID, Payload: normalized})
		},
	)

	// 清理会话资源
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

	// 发布管道错误事件
	if err != nil && err != context.Canceled {
		h.bus.Publish(bus.Event{Name: "error", SessionID: sessionID, TraceID: traceID, Payload: map[string]string{"code": "mic_pipeline_failed", "message": err.Error()}})
	}
}

// clearSession 清理会话资源
func (h *ConversationHandler) clearSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// 取消对话上下文
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	// 关闭音频流
	if stream := h.streams[sessionID]; stream != nil {
		stream.Close()
	}
	// 关闭 WebSocket 连接
	if conn := h.sockets[sessionID]; conn != nil {
		_ = conn.Close()
	}
	// 关闭音频源并禁用后台捕获
	if src := h.sources[sessionID]; src != nil {
		if bg, ok := src.(audio.BackgroundCaptureController); ok {
			bg.EnableBackgroundCapture(sessionID, true)
		}
		if closable, ok := src.(interface{ Close() error }); ok {
			_ = closable.Close()
		}
	}
	// 删除所有会话相关数据
	delete(h.streams, sessionID)
	delete(h.sources, sessionID)
	delete(h.sockets, sessionID)
	delete(h.capture, sessionID)
	delete(h.modes, sessionID)
	delete(h.frameSeen, sessionID)
	// 如果清理的是当前活跃会话，重置状态
	if h.sid == sessionID {
		h.active = false
		h.sid = ""
		h.tid = ""
	}
}

// resolvePolicy 解析对话策略
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

// normalizeConversationMode 规范化对话模式
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
