package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"norma/backend/core/audio"
	"norma/backend/core/llm"
	"norma/backend/core/memory"
	"norma/backend/core/runtime/bus"
)

// Orchestrator 编排器，连接 ASR/RAG/LLM/TTS 与事件总线
// 负责语音会话的完整流程编排：音频输入 -> ASR 识别 -> RAG 检索 -> LLM 生成 -> TTS 合成 -> 播放
type Orchestrator struct {
	rt        *Runtime         // 运行时实例
	retriever memory.Retriever // RAG 检索器
	llm       llm.Streamer     // LLM 流式输出
	tts       audio.TTS        // TTS 语音合成
	ragTopK   int              // RAG 检索返回结果数量
	playback  playbackSink     // 音频播放接收器

	mu      sync.Mutex                    // 互斥锁，保护 cancels
	cancels map[string]context.CancelFunc // 会话取消函数映射
}

// playbackSink 音频播放接收器接口
type playbackSink interface {
	Queue(sessionID, traceID string, chunk []byte) error // 入队音频数据
	Complete(sessionID, traceID string)                  // 完成播放
	Stop(sessionID, traceID, reason string)              // 停止播放
	Drain(ctx context.Context, sessionID string) error   // 排空队列
	Wait(ctx context.Context, sessionID string) error    // 等待播放完成
}

// NewOrchestrator 创建新的编排器（简化版本，使用默认播放接收器）
func NewOrchestrator(
	rt *Runtime,
	retriever memory.Retriever,
	llm llm.Streamer,
	tts audio.TTS,
	ragTopK int,
) *Orchestrator {
	return NewOrchestratorWithPlayback(rt, retriever, llm, tts, ragTopK, nil)
}

// NewOrchestratorWithPlayback 创建带播放接收器的编排器
func NewOrchestratorWithPlayback(
	rt *Runtime,
	retriever memory.Retriever,
	llm llm.Streamer,
	tts audio.TTS,
	ragTopK int,
	playback playbackSink,
) *Orchestrator {
	if ragTopK <= 0 {
		ragTopK = 4 // 默认 RAG 检索数量
	}
	o := &Orchestrator{
		rt:        rt,
		retriever: retriever,
		llm:       llm,
		tts:       tts,
		ragTopK:   ragTopK,
		playback:  playback,
		cancels:   make(map[string]context.CancelFunc),
	}

	// 订阅 ASR 识别完成事件，启动 LLM 生成
	o.rt.Bus.Subscribe("asr.final", o.onASRFinal)
	// 订阅 LLM token 生成事件，处理流式输出
	o.rt.Bus.Subscribe("llm.token", o.onLLMToken)
	// 订阅 LLM 完成事件，启动 TTS 合成
	o.rt.Bus.Subscribe("llm.done", o.onLLMDone)
	// 订阅 TTS chunk 事件，推送播放
	o.rt.Bus.Subscribe("tts.chunk", o.onTTSChunk)
	// 订阅打断事件，取消在途会话
	o.rt.Bus.Subscribe("interrupt", o.onInterrupt)

	return o
}

// onASRFinal 处理 ASR 识别完成事件
// 流程：检索 RAG -> 构建 prompt -> 启动 LLM 流式生成
func (o *Orchestrator) onASRFinal(evt bus.Event) {
	text, ok := evt.Payload.(string)
	if !ok || text == "" {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	sessionID, traceID := evt.SessionID, evt.TraceID

	o.mu.Lock()
	o.cancels[sessionID] = cancel
	o.mu.Unlock()

	// RAG 检索获取相关上下文
	topK := o.ragTopK
	var chunks []memory.Chunk
	var err error
	chunks, err = o.retriever.Search(ctx, text, topK)
	if err != nil {
		log.Printf("[orchestrator] retriever search failed: %v", err)
	}

	// 构建 prompt 和 context
	var prompt string
	var contextStrings []string
	if len(chunks) > 0 {
		var b strings.Builder
		b.WriteString("相关上下文：\n")
		for _, c := range chunks {
			b.WriteString(c.Content)
			b.WriteString("\n---\n")
			contextStrings = append(contextStrings, c.Content)
		}
		b.WriteString("用户问题：")
		b.WriteString(text)
		prompt = b.String()
	} else {
		prompt = text
	}

	// 发布 RAG 准备就绪事件
	o.rt.Bus.Publish(bus.Event{
		Name:      "rag.ready",
		SessionID: sessionID,
		TraceID:   traceID,
		Payload:   len(chunks),
	})

	// 调用 LLM 流式生成
	llmCtx, llmCancel := context.WithCancel(ctx)
	o.mu.Lock()
	o.cancels[sessionID+"_llm"] = llmCancel
	o.mu.Unlock()

	req := llm.Request{
		SessionID: sessionID,
		Prompt:    prompt,
		Context:   contextStrings,
	}

	err = o.llm.Stream(llmCtx, req, func(token string) {
		o.rt.Bus.Publish(bus.Event{
			Name:      "llm.token",
			SessionID: sessionID,
			TraceID:   traceID,
			Payload:   token,
		})
	})
	if err != nil {
		log.Printf("[orchestrator] llm stream failed: %v", err)
		o.rt.Bus.Publish(bus.Event{
			Name:      "error",
			SessionID: sessionID,
			TraceID:   traceID,
			Payload:   err.Error(),
		})
		return
	}

	// 发布 LLM 完成事件
	o.rt.Bus.Publish(bus.Event{
		Name:      "llm.done",
		SessionID: sessionID,
		TraceID:   traceID,
	})
	if err != nil {
		log.Printf("[orchestrator] llm stream failed: %v", err)
		o.rt.Bus.Publish(bus.Event{
			Name:      "error",
			SessionID: sessionID,
			TraceID:   traceID,
			Payload:   err.Error(),
		})
	}
}

// onLLMToken 处理 LLM token 生成事件
// 触发 TTS 合成并推送播放
func (o *Orchestrator) onLLMToken(evt bus.Event) {
	text, ok := evt.Payload.(string)
	if !ok || text == "" {
		return
	}

	// 调用 TTS 合成
	err := o.tts.Stream(context.Background(), text, func(chunk []byte) {
		o.rt.Bus.Publish(bus.Event{
			Name:      "tts.chunk",
			SessionID: evt.SessionID,
			TraceID:   evt.TraceID,
			Payload:   base64.StdEncoding.EncodeToString(chunk),
		})
		if o.playback != nil {
			_ = o.playback.Queue(evt.SessionID, evt.TraceID, chunk)
		}
	})
	if err != nil {
		log.Printf("[orchestrator] tts stream failed: %v", err)
	}
}

// onLLMDone 处理 LLM 生成完成事件
// 通知播放完成
func (o *Orchestrator) onLLMDone(evt bus.Event) {
	if o.playback != nil {
		o.playback.Complete(evt.SessionID, evt.TraceID)
	}
}

// onTTSChunk 处理 TTS 音频 chunk 事件
func (o *Orchestrator) onTTSChunk(evt bus.Event) {
	// 事件已在上游处理，这里保留用于扩展
}

// onInterrupt 处理打断事件
// 取消对应会话的所有在途操作
func (o *Orchestrator) onInterrupt(evt bus.Event) {
	sessionID := evt.SessionID

	o.mu.Lock()
	if cancel, ok := o.cancels[sessionID]; ok {
		cancel()
		delete(o.cancels, sessionID)
	}
	if cancel, ok := o.cancels[sessionID+"_llm"]; ok {
		cancel()
		delete(o.cancels, sessionID+"_llm")
	}
	o.mu.Unlock()

	// 停止播放
	if o.playback != nil {
		o.playback.Stop(sessionID, evt.TraceID, "interrupt")
	}
}

// StartSession 启动新的语音会话
// 返回会话 ID 和追踪 ID
func (o *Orchestrator) StartSession() (string, string) {
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	traceID := fmt.Sprintf("%s-%d", sessionID, time.Now().UnixMilli())
	return sessionID, traceID
}

// CancelSession 取消指定会话
func (o *Orchestrator) CancelSession(sessionID string) {
	o.mu.Lock()
	if cancel, ok := o.cancels[sessionID]; ok {
		cancel()
		delete(o.cancels, sessionID)
	}
	if cancel, ok := o.cancels[sessionID+"_llm"]; ok {
		cancel()
		delete(o.cancels, sessionID+"_llm")
	}
	o.mu.Unlock()

	if o.playback != nil {
		o.playback.Stop(sessionID, "", "cancel")
	}
}
