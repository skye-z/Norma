package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"norma/backend/core/audio"
	"norma/backend/core/llm"
	"norma/backend/core/memory"
	"norma/backend/core/runtime/bus"
)

// Orchestrator 编排器，连接记忆检索、大模型、语音合成与播放
// 一次语音问答对应一个会话，会话内的各阶段共享同一个可取消上下文
type Orchestrator struct {
	rt        *Runtime
	retriever memory.Retriever
	llm       llm.Streamer
	tts       audio.TTS
	ragTopK   int
	playback  playbackSink
	persona   string
	memory    memoryWriter

	// 最近几轮对话，让模型知道前面聊过什么，也不会每一轮都当作第一次见面
	hmu         sync.Mutex
	history     []llm.Turn
	historyAt   time.Time
	historyMax  int
	historyIdle time.Duration
	now         func() time.Time

	mu       sync.Mutex
	sessions map[string]*session
}

// 记忆写入接口，用于响应“记住……”之类的口语指令
type memoryWriter interface {
	Remember(ctx context.Context, fact string) error
}

// 播放接收器接口
type playbackSink interface {
	Queue(sessionID, traceID string, chunk []byte) error
	Complete(sessionID, traceID string)
	Stop(sessionID, traceID, reason string)
	Drain(ctx context.Context, sessionID string) error
	Wait(ctx context.Context, sessionID string) error
}

// 单个会话的运行状态
type session struct {
	id      string
	trace   string
	cancel  context.CancelFunc
	done    chan struct{}
	stopped bool
}

// 创建编排器，不带播放接收器
func NewOrchestrator(
	rt *Runtime,
	retriever memory.Retriever,
	llm llm.Streamer,
	tts audio.TTS,
	ragTopK int,
) *Orchestrator {
	return NewOrchestratorWithPlayback(rt, retriever, llm, tts, ragTopK, nil)
}

// 创建带播放接收器的编排器
func NewOrchestratorWithPlayback(
	rt *Runtime,
	retriever memory.Retriever,
	llm llm.Streamer,
	tts audio.TTS,
	ragTopK int,
	playback playbackSink,
) *Orchestrator {
	if ragTopK <= 0 {
		ragTopK = 4
	}
	o := &Orchestrator{
		rt:        rt,
		retriever: retriever,
		llm:       llm,
		tts:       tts,
		ragTopK:   ragTopK,
		playback:  playback,
		sessions:  make(map[string]*session),

		historyMax:  8,
		historyIdle: 15 * time.Minute,
		now:         time.Now,
	}
	o.rt.Bus.Subscribe("asr.final", o.onASRFinal)
	o.rt.Bus.Subscribe("interrupt", o.onInterrupt)
	return o
}

// 设置角色设定，随每次请求发送给大模型
func (o *Orchestrator) SetPersona(persona string) {
	o.mu.Lock()
	o.persona = persona
	o.mu.Unlock()
}

// 设置记忆写入器；未设置时“记住……”只当作普通提问
func (o *Orchestrator) SetMemory(w memoryWriter) {
	o.mu.Lock()
	o.memory = w
	o.mu.Unlock()
}

// 识别完成后启动会话；同一会话号的在途任务会先被取消并等待退出
func (o *Orchestrator) onASRFinal(evt bus.Event) {
	text, ok := evt.Payload.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return
	}
	o.cancelAndWait(evt.SessionID, evt.TraceID, "superseded")

	ctx, cancel := context.WithCancel(context.Background())
	s := &session{id: evt.SessionID, trace: evt.TraceID, cancel: cancel, done: make(chan struct{})}
	o.mu.Lock()
	o.sessions[s.id] = s
	persona := o.persona
	o.mu.Unlock()

	go func() {
		defer close(s.done)
		o.run(ctx, s, text, persona)
	}()
}

// 会话主流程：检索、生成、分句合成、播放
func (o *Orchestrator) run(ctx context.Context, s *session, text, persona string) {
	pub := func(name string, payload any) {
		if ctx.Err() != nil {
			return
		}
		o.rt.Bus.Publish(bus.Event{Name: name, SessionID: s.id, TraceID: s.trace, Payload: payload})
	}

	var contexts []string
	o.mu.Lock()
	mem := o.memory
	o.mu.Unlock()
	if fact, ok := memory.ParseRememberCommand(text); ok && mem != nil {
		if err := mem.Remember(ctx, fact); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[orchestrator] remember failed: %v", err)
			contexts = append(contexts, "（系统提示：用户想让你记住“"+fact+"”，但保存失败了，请如实告诉用户。）")
		} else {
			contexts = append(contexts, "（系统提示：已把“"+fact+"”写入长期记忆，请简短确认，不要复述全文。）")
			pub("memory.saved", fact)
		}
	}
	chunks, err := o.retriever.Search(ctx, text, o.ragTopK)
	if err != nil && ctx.Err() == nil {
		log.Printf("[orchestrator] retriever search failed: %v", err)
	}
	for _, c := range chunks {
		contexts = append(contexts, c.Content)
	}
	pub("rag.ready", len(chunks))
	if ctx.Err() != nil {
		return
	}

	sentences := make(chan string, 16)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.speak(ctx, s, sentences, pub)
	}()

	splitter := newSentenceSplitter()
	req := llm.Request{SessionID: s.id, Prompt: text, Context: contexts, Persona: persona, History: o.historySnapshot()}
	var reply strings.Builder
	llmErr := o.llm.Stream(ctx, req, func(token string) {
		if ctx.Err() != nil {
			return
		}
		reply.WriteString(token)
		pub("llm.token", token)
		for _, sent := range splitter.Push(token) {
			select {
			case sentences <- sent:
			case <-ctx.Done():
				return
			}
		}
	})
	if ctx.Err() == nil {
		if rest := splitter.Flush(); rest != "" {
			select {
			case sentences <- rest:
			case <-ctx.Done():
			}
		}
	}
	close(sentences)
	wg.Wait()

	if ctx.Err() != nil {
		return
	}
	if llmErr != nil {
		log.Printf("[orchestrator] llm stream failed: %v", llmErr)
		pub("error", map[string]string{"code": "llm_stream_failed", "message": llmErr.Error()})
		o.finish(s)
		return
	}
	o.recordTurn(text, reply.String())
	pub("llm.finished", nil)
	if o.playback != nil {
		o.playback.Complete(s.id, s.trace)
		if err := o.playback.Wait(ctx, s.id); err != nil && ctx.Err() == nil {
			log.Printf("[orchestrator] playback wait: %v", err)
		}
	}
	if ctx.Err() != nil {
		return
	}
	pub("llm.done", nil)
	pub("session.done", nil)
	o.finish(s)
}

// 按顺序逐句合成；每个分句一次合成调用，输出块立即入播放队列
func (o *Orchestrator) speak(ctx context.Context, s *session, sentences <-chan string, pub func(string, any)) {
	for sent := range sentences {
		if ctx.Err() != nil {
			continue
		}
		pub("llm.sentence", sent)
		err := o.tts.Stream(ctx, sent, func(chunk []byte) {
			if ctx.Err() != nil {
				return
			}
			pub("tts.chunk.ready", len(chunk))
			pub("tts.chunk", base64.StdEncoding.EncodeToString(chunk))
			if o.playback != nil {
				_ = o.playback.Queue(s.id, s.trace, chunk)
			}
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("[orchestrator] tts stream failed: %v", err)
			pub("tts.error", err.Error())
		}
	}
}

func (o *Orchestrator) finish(s *session) {
	o.mu.Lock()
	if o.sessions[s.id] == s {
		delete(o.sessions, s.id)
	}
	o.mu.Unlock()
}

// 打断：取消上下文、停止合成与播放，并等待在途协程退出后再报告完成
func (o *Orchestrator) onInterrupt(evt bus.Event) {
	started := time.Now()
	if !o.cancelAndWait(evt.SessionID, evt.TraceID, "interrupt") {
		// 没有在途会话时仍要幂等地停掉合成与播放，排空上一轮残留
		_ = o.tts.Stop(evt.SessionID)
		if o.playback != nil {
			o.playback.Stop(evt.SessionID, evt.TraceID, "interrupt")
		}
		return
	}
	o.rt.Bus.Publish(bus.Event{
		Name:      "interrupt.done",
		SessionID: evt.SessionID,
		TraceID:   evt.TraceID,
		Payload:   time.Since(started).Microseconds(),
	})
}

// 取消指定会话并等待其退出，返回是否存在在途会话
func (o *Orchestrator) cancelAndWait(sessionID, traceID, reason string) bool {
	o.mu.Lock()
	s, ok := o.sessions[sessionID]
	if ok {
		delete(o.sessions, sessionID)
		s.stopped = true
	}
	o.mu.Unlock()
	if !ok {
		return false
	}
	s.cancel()
	_ = o.tts.Stop(sessionID)
	if o.playback != nil {
		o.playback.Stop(sessionID, traceID, reason)
	}
	<-s.done
	return true
}

// StartSession 生成新的会话号和追踪号
func (o *Orchestrator) StartSession() (string, string) {
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	traceID := fmt.Sprintf("%s-%d", sessionID, time.Now().UnixMilli())
	return sessionID, traceID
}

// 取消指定会话
func (o *Orchestrator) CancelSession(sessionID string) {
	o.cancelAndWait(sessionID, "", "cancel")
}

var liveMarkRe = regexp.MustCompile(`\{\{[^}]*\}\}?`)

// historySnapshot 返回仍有效的历史副本；超过空闲时间没有对话就视为新的对话，旧历史作废
func (o *Orchestrator) historySnapshot() []llm.Turn {
	o.hmu.Lock()
	defer o.hmu.Unlock()
	if len(o.history) == 0 {
		return nil
	}
	if o.now().Sub(o.historyAt) > o.historyIdle {
		o.history = nil
		return nil
	}
	return append([]llm.Turn(nil), o.history...)
}

// recordTurn 在一轮对话正常结束后记录；Live2D 标识不属于正文，不进入历史。
// 被打断或失败的轮次不会走到这里，避免把说了一半的回复当成完整回答
func (o *Orchestrator) recordTurn(user, assistant string) {
	assistant = strings.TrimSpace(liveMarkRe.ReplaceAllString(assistant, ""))
	user = strings.TrimSpace(user)
	if user == "" || assistant == "" {
		return
	}
	o.hmu.Lock()
	defer o.hmu.Unlock()
	if len(o.history) > 0 && o.now().Sub(o.historyAt) > o.historyIdle {
		o.history = nil
	}
	o.history = append(o.history, llm.Turn{User: user, Assistant: assistant})
	if len(o.history) > o.historyMax {
		o.history = append([]llm.Turn(nil), o.history[len(o.history)-o.historyMax:]...)
	}
	o.historyAt = o.now()
}
