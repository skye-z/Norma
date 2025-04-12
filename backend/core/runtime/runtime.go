package runtime

import (
	"log"

	"norma/backend/core/runtime/bus"
	"norma/backend/core/runtime/state"
)

// Runtime 运行时核心组件，负责状态管理和事件总线
type Runtime struct {
	Bus       *bus.Bus       // 事件总线，用于组件间通信
	State     *state.Machine // 状态机，管理会话状态转换
	Inspector *Inspector     // 检查器，用于运行时诊断
}

// New 创建新的运行时实例
// 初始化事件总线、状态机，并订阅核心事件
func New() *Runtime {
	inspector := NewInspector(80)
	rt := &Runtime{
		Bus:       bus.New(),
		State:     state.NewMachine(),
		Inspector: inspector,
	}
	_ = inspector.Bind(rt.Bus)
	inspector.SetState(string(rt.State.Get()))

	// 监听音频输入开始事件
	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		// 打断检测：正在说话时收到新音频输入，触发打断
		if rt.State.Get() == state.Speaking {
			log.Printf("[runtime] barge-in trigger while speaking session=%s trace=%s", evt.SessionID, evt.TraceID)
			rt.Bus.Publish(bus.Event{Name: "interrupt", SessionID: evt.SessionID, TraceID: evt.TraceID, Payload: "barge_in"})
		}
		// 状态转换为倾听模式
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] audio.vad.start session=%s trace=%s", evt.SessionID, evt.TraceID)
	})

	// 监听 ASR 部分识别结果
	rt.Bus.Subscribe("asr.partial", func(evt bus.Event) {
		if text, ok := evt.Payload.(string); ok {
			log.Printf("[runtime] asr.partial session=%s trace=%s text=%q", evt.SessionID, evt.TraceID, text)
		}
		_ = evt
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
	})

	// 监听 ASR 最终识别结果，转换为思考状态
	rt.Bus.Subscribe("asr.final", func(evt bus.Event) {
		rt.State.Set(state.Thinking)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] asr.final -> Thinking session=%s trace=%s text=%v", evt.SessionID, evt.TraceID, evt.Payload)
	})

	// 监听 LLM token 生成，转换为说话状态
	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		if text, ok := evt.Payload.(string); ok {
			log.Printf("[runtime] llm.token session=%s trace=%s token=%q", evt.SessionID, evt.TraceID, text)
		}
		_ = evt
		rt.State.Set(state.Speaking)
		inspector.SetState(string(rt.State.Get()))
	})

	// 监听 LLM 完成事件
	rt.Bus.Subscribe("llm.done", func(evt bus.Event) {
		_ = evt
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
	})

	// 监听打断事件
	rt.Bus.Subscribe("interrupt", func(evt bus.Event) {
		rt.State.Set(state.Interrupted)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] interrupt received: session=%s trace=%s", evt.SessionID, evt.TraceID)
		// 打断处理完成后恢复到倾听状态
		rt.State.Set(state.Listening)
		inspector.SetState(string(rt.State.Get()))
	})

	// 监听错误事件
	rt.Bus.Subscribe("error", func(evt bus.Event) {
		rt.State.Set(state.Error)
		inspector.SetState(string(rt.State.Get()))
		log.Printf("[runtime] error event: session=%s trace=%s payload=%v", evt.SessionID, evt.TraceID, evt.Payload)
		// 错误处理完成后恢复到空闲状态
		rt.State.Set(state.Idle)
		inspector.SetState(string(rt.State.Get()))
	})

	return rt
}
