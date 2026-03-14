// Package api 提供 HTTP API
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"norma/backend/core/live2d"
	"norma/backend/core/memory"
	"norma/backend/core/providers"
	"norma/backend/core/runtime"
	"norma/backend/core/runtime/bus"
	"norma/backend/utils"
	"norma/pkg/layout"
)

// 诊断处理器
type DiagnosticsHandler struct {
	configPath string
	modelDir   string
	bus        *bus.Bus
	inspector  *runtime.Inspector
	memorySvc  *memory.Service
}

// diagnosticsRunRequest 诊断请求
type diagnosticsRunRequest struct {
	Text   string  `json:"text"`
	Motion string  `json:"motion"`
	Stage  string  `json:"stage"`
	EyeX   float64 `json:"eye_x"`
	EyeY   float64 `json:"eye_y"`
}

// diagnosticsRunResponse 诊断响应
type diagnosticsRunResponse struct {
	Target  string         `json:"target"`
	OK      bool           `json:"ok"`
	Summary string         `json:"summary"`
	Details map[string]any `json:"details,omitempty"`
	State   string         `json:"state,omitempty"`
}

// 创建诊断处理器
func NewDiagnosticsHandler(configPath, modelDir string, b *bus.Bus, inspector *runtime.Inspector, memorySvc *memory.Service) *DiagnosticsHandler {
	return &DiagnosticsHandler{configPath: configPath, modelDir: modelDir, bus: b, inspector: inspector, memorySvc: memorySvc}
}

// 注册路由
func (h *DiagnosticsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/diagnostics/run/", h.handleRun)
}

// handleRun 运行诊断
func (h *DiagnosticsHandler) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	target := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/diagnostics/run/"))
	if target == "" {
		writeErr(w, http.StatusBadRequest, "target required")
		return
	}

	var req diagnosticsRunRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
	}

	var result diagnosticsRunResponse
	switch target {
	case "live2d":
		result = h.runLive2D(req)
	case "tts":
		result = h.runTTS(req)
	case "asr":
		result = h.runASR(req)
	case "llm":
		result = h.runLLM(req)
	case "memory":
		result = h.runMemory(req)
	case "rag":
		result = h.runRAG(req)
	default:
		writeErr(w, http.StatusNotFound, "unknown diagnostics target")
		return
	}

	if h.inspector != nil {
		result.State = h.inspector.Snapshot().State
	}
	writeJSON(w, http.StatusOK, result)
}

// runLive2D 诊断 Live2D
func (h *DiagnosticsHandler) runLive2D(req diagnosticsRunRequest) diagnosticsRunResponse {
	modelDir := strings.TrimSpace(h.modelDir)
	if modelDir == "" {
		modelDir = filepath.ToSlash(layout.ModelDir)
	}
	validation := live2d.ValidateModelDir(modelDir)
	motion := strings.TrimSpace(req.Motion)
	if motion == "" {
		motion = "Tap"
	}
	if validation.OK() && h.bus != nil {
		h.bus.Publish(bus.Event{Name: "live2d.motion", SessionID: "diagnostics", TraceID: "diag-live2d", Payload: map[string]any{"group": motion, "index": 0}})
		h.bus.Publish(bus.Event{Name: "live2d.eye", SessionID: "diagnostics", TraceID: "diag-live2d", Payload: map[string]any{"x": req.EyeX, "y": req.EyeY}})
	}
	return diagnosticsRunResponse{
		Target:  "live2d",
		OK:      validation.OK(),
		Summary: map[bool]string{true: "Live2D 模型资源有效，已由后端发出动作测试事件", false: "Live2D 模型资源未通过校验"}[validation.OK()],
		Details: map[string]any{"model_dir": validation.ModelDir, "issues": validation.Issues, "motion": motion, "eye_x": req.EyeX, "eye_y": req.EyeY},
	}
}

// runTTS 诊断 TTS
func (h *DiagnosticsHandler) runTTS(req diagnosticsRunRequest) diagnosticsRunResponse {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		return diagnosticsRunResponse{Target: "tts", OK: false, Summary: "读取配置失败", Details: map[string]any{"error": err.Error()}}
	}
	binaryOK := pathExists(cfg.TTS.BinaryPath)
	modelOK := pathExists(cfg.TTS.ModelPath)
	ready := binaryOK && modelOK
	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "这是后端 TTS 联调测试。"
	}
	if ready && h.bus != nil {
		h.bus.Publish(bus.Event{Name: "diagnostics.tts.requested", SessionID: "diagnostics", TraceID: "diag-tts", Payload: map[string]any{"text": text, "provider": cfg.TTS.Provider}})
	}
	return diagnosticsRunResponse{
		Target:  "tts",
		OK:      ready,
		Summary: map[bool]string{true: "TTS 配置与资源可用，后端已发出联调请求", false: "TTS 缺少可用资源"}[ready],
		Details: map[string]any{"provider": cfg.TTS.Provider, "voice": cfg.TTS.Voice, "binary_ok": binaryOK, "model_ok": modelOK, "text": text},
	}
}

// runASR 诊断 ASR
func (h *DiagnosticsHandler) runASR(req diagnosticsRunRequest) diagnosticsRunResponse {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		return diagnosticsRunResponse{Target: "asr", OK: false, Summary: "读取配置失败", Details: map[string]any{"error": err.Error()}}
	}
	binaryOK := pathExists(cfg.ASR.BinaryPath)
	modelOK := pathExists(cfg.ASR.ModelPath)
	ready := binaryOK && modelOK
	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "这是后端 ASR 联调测试。"
	}
	stage := strings.TrimSpace(req.Stage)
	if stage == "" {
		stage = "final"
	}
	if ready && h.bus != nil {
		eventName := "asr.final"
		if strings.EqualFold(stage, "partial") {
			eventName = "asr.partial"
		}
		h.bus.Publish(bus.Event{Name: eventName, SessionID: "diagnostics", TraceID: "diag-asr", Payload: text})
	}
	return diagnosticsRunResponse{
		Target:  "asr",
		OK:      ready,
		Summary: map[bool]string{true: "ASR 配置与资源可用，后端已发出联调请求", false: "ASR 缺少可用资源"}[ready],
		Details: map[string]any{"provider": cfg.ASR.Provider, "binary_ok": binaryOK, "model_ok": modelOK, "stage": stage, "text": text},
	}
}

// runLLM 诊断 LLM
func (h *DiagnosticsHandler) runLLM(req diagnosticsRunRequest) diagnosticsRunResponse {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		return diagnosticsRunResponse{Target: "llm", OK: false, Summary: "读取配置失败", Details: map[string]any{"error": err.Error()}}
	}
	ok, detail := endpointReachable(cfg.LLM.Endpoint)
	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "请返回：诊断链路正常。"
	}
	if ok && h.bus != nil {
		h.bus.Publish(bus.Event{Name: "diagnostics.llm.requested", SessionID: "diagnostics", TraceID: "diag-llm", Payload: map[string]any{"text": text, "provider": cfg.LLM.Provider, "model": cfg.LLM.Model}})
	}
	return diagnosticsRunResponse{
		Target:  "llm",
		OK:      ok,
		Summary: map[bool]string{true: "LLM 端点可达，后端已记录联调请求", false: "LLM 端点不可达"}[ok],
		Details: map[string]any{"provider": cfg.LLM.Provider, "model": cfg.LLM.Model, "endpoint": cfg.LLM.Endpoint, "endpoint_ok": ok, "endpoint_detail": detail, "text": text},
	}
}

// runMemory 诊断记忆服务
func (h *DiagnosticsHandler) runMemory(req diagnosticsRunRequest) diagnosticsRunResponse {
	if h.memorySvc == nil {
		return diagnosticsRunResponse{Target: "memory", OK: false, Summary: "记忆服务不可用"}
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = fmt.Sprintf("diagnostics memory probe %d", time.Now().Unix())
	}
	ctx := context.Background()
	created, err := h.memorySvc.AddDetailed(ctx, memory.AddInput{Content: text, Kind: "fact", Importance: 3, MemoryType: "diagnostic", Source: "diagnostics", Evidence: "backend diagnostics probe"})
	if err != nil {
		return diagnosticsRunResponse{Target: "memory", OK: false, Summary: "写入记忆失败", Details: map[string]any{"error": err.Error()}}
	}
	hits, err := h.memorySvc.Search(ctx, text, 5)
	if err != nil {
		return diagnosticsRunResponse{Target: "memory", OK: false, Summary: "写入成功但搜索失败", Details: map[string]any{"created_id": created.ID, "error": err.Error()}}
	}
	_ = h.memorySvc.Delete(ctx, created.ID)
	return diagnosticsRunResponse{
		Target:  "memory",
		OK:      len(hits) > 0,
		Summary: map[bool]string{true: "记忆写入与检索联调成功", false: "记忆写入成功但未命中检索结果"}[len(hits) > 0],
		Details: map[string]any{"probe_text": text, "created_id": created.ID, "hits": len(hits)},
	}
}

// runRAG 诊断 RAG
func (h *DiagnosticsHandler) runRAG(req diagnosticsRunRequest) diagnosticsRunResponse {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		return diagnosticsRunResponse{Target: "rag", OK: false, Summary: "读取配置失败", Details: map[string]any{"error": err.Error()}}
	}
	providerMgr := providers.NewManager("")
	health := providers.BuildHealth(providerMgr)
	endpointOK, endpointDetail := endpointReachable(cfg.RAG.EmbedEndpoint)
	ready := endpointOK || health["embedding"].Status == "ready"
	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "知识库联调测试"
	}
	topK := 4
	if ready && h.bus != nil {
		h.bus.Publish(bus.Event{Name: "diagnostics.rag.requested", SessionID: "diagnostics", TraceID: "diag-rag", Payload: map[string]any{"query": text, "top_k": topK}})
	}
	return diagnosticsRunResponse{
		Target:  "rag",
		OK:      ready,
		Summary: map[bool]string{true: "知识库/检索配置可用，后端已记录联调请求", false: "知识库/检索缺少可用 embedding 资源或端点不可达"}[ready],
		Details: map[string]any{"embed_provider": cfg.RAG.EmbedProvider, "embed_model": cfg.RAG.EmbedModel, "embed_endpoint": cfg.RAG.EmbedEndpoint, "endpoint_ok": endpointOK, "endpoint_detail": endpointDetail, "embedding_health": health["embedding"].Status, "top_k": topK, "query": text},
	}
}

// pathExists 检查路径是否存在
func pathExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// endpointReachable 检查端点是否可达
func endpointReachable(raw string) (bool, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, "endpoint empty"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return false, "endpoint invalid"
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), 1500*time.Millisecond)
	if err != nil {
		return false, err.Error()
	}
	_ = conn.Close()
	return true, "tcp reachable"
}

// firstNonEmpty 返回第一个非空字符串
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
