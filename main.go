// Package main 是 Norma 应用程序的主入口包。
// 该程序是一个桌面AI助手应用，支持语音交互、LLM对话、TTS语音合成等功能。
// 可以以桌面应用模式运行，也可以以演示模式运行事件链演示。
package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"time"

	"norma/backend/core/audio"
	"norma/backend/core/bootstrap"
	"norma/backend/core/db"
	"norma/backend/core/desktop"
	"norma/backend/core/providers"
	"norma/backend/core/runtime"
	"norma/backend/core/runtime/bus"
	"norma/backend/utils"
	"norma/backend/utils/ops"
	"norma/pkg/layout"
)

// frontendAssets 嵌入前端构建产物，用于桌面应用模式
//
//go:embed all:frontend/dist
var frontendAssets embed.FS

var (
	version = "dev"     // 程序版本号
	commit  = "unknown" // Git提交哈希
)

// main 是应用程序的入口函数。
// 支持两种运行模式：
// 1. 桌面应用模式（默认）：启动桌面窗口应用
// 2. 演示模式 (-demo)：运行事件链演示并退出
func main() {
	// 解析命令行参数
	cfgPath := flag.String("config", layout.ConfigFileName, "path to config file")
	demo := flag.Bool("demo", false, "run event-chain demo and exit")
	flag.Parse()

	// 非演示模式：启动桌面应用
	if !*demo {
		if err := desktop.Run(frontendAssets); err != nil {
			log.Fatalf("start desktop app: %v", err)
		}
		return
	}

	// 演示模式：加载配置并初始化运行时
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 初始化提供者管理器，确保必要的目录布局
	providerMgr := providers.NewManager(cfg.Providers.Root)
	if err := providerMgr.EnsureLayout(); err != nil {
		log.Printf("[providers] ensure layout failed: %v", err)
	}
	// 打印嵌入的语音模型支持信息
	log.Printf("[build] embedded whisper: %v", audio.EmbeddedWhisperEnabled())
	log.Printf("[build] embedded sherpa: %v", audio.EmbeddedSherpaEnabled())

	// 初始化日志路由，将日志写入文件
	logCloser, err := ops.SetupLogRouting(cfg.App.LogDir)
	if err != nil {
		log.Fatalf("init log routing: %v", err)
	}
	defer func() {
		if cerr := logCloser.Close(); cerr != nil {
			log.Printf("close log routing: %v", cerr)
		}
	}()

	// 初始化数据库引擎
	engine, err := db.NewEngine(cfg.Database.Path, cfg.App.Env == "dev")
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer engine.Close()

	// 创建运行时堆栈，包含所有核心服务
	stack, err := bootstrap.NewStack(cfg, engine)
	if err != nil {
		log.Fatalf("init runtime stack: %v", err)
	}
	rt := stack.Runtime

	// 订阅音频相关事件，用于日志记录
	rt.Bus.Subscribe("audio.vad.start", func(evt bus.Event) {
		log.Printf("[audio] vad.start session=%s", evt.SessionID)
	})
	rt.Bus.Subscribe("audio.vad.stop", func(evt bus.Event) {
		log.Printf("[audio] vad.stop session=%s", evt.SessionID)
	})
	rt.Bus.Subscribe("audio.play.start", func(evt bus.Event) {
		log.Printf("[audio] play.start payload=%v", evt.Payload)
	})
	rt.Bus.Subscribe("audio.play.end", func(evt bus.Event) {
		log.Printf("[audio] play.end payload=%v", evt.Payload)
	})
	// 订阅错误事件
	rt.Bus.Subscribe("error", func(evt bus.Event) {
		log.Printf("[error] payload=%v", evt.Payload)
	})

	// 订阅RAG和ASR相关事件，用于演示日志
	rt.Bus.Subscribe("rag.ready", func(evt bus.Event) {
		log.Printf("[demo] rag.ready chunks=%v", evt.Payload)
	})
	rt.Bus.Subscribe("asr.partial", func(evt bus.Event) {
		if s, ok := evt.Payload.(string); ok {
			log.Printf("[demo] asr.partial=%q", s)
		}
	})
	rt.Bus.Subscribe("llm.token", func(evt bus.Event) {
		if s, ok := evt.Payload.(string); ok {
			log.Printf("[demo] llm.token=%q", s)
		}
	})
	rt.Bus.Subscribe("tts.chunk.ready", func(evt bus.Event) {
		log.Printf("[demo] tts.chunk bytes=%v", evt.Payload)
	})
	// 订阅Live2D相关事件
	rt.Bus.Subscribe("live2d.motion", func(evt bus.Event) {
		log.Printf("[live2d] motion=%v", evt.Payload)
	})
	rt.Bus.Subscribe("live2d.mouth", func(evt bus.Event) {
		log.Printf("[live2d] mouth=%v", evt.Payload)
	})

	// 打印启动信息
	log.Printf("[%s] started env=%s version=%s commit=%s db=%s state=%s", cfg.App.Name, cfg.App.Env, version, commit, cfg.Database.Path, rt.State.Get())

	// 运行演示输入管道
	if *demo {
		runDemoInputPipeline(cfg, rt, providerMgr)
		// 等待演示完成
		time.Sleep(6 * time.Second)
		log.Printf("[demo] final state=%s alerts=%d", rt.State.Get(), len(stack.Tracker.Snapshot().Alerts))
		return
	}

	// 桌面模式：保持运行
	select {}
}

// runDemoInputPipeline 运行演示模式的语音输入管道。
// 该函数创建一个模拟音频源，并通过语音管道处理音频数据，
// 模拟真实的语音交互流程。
//
// 参数：
//   - cfg: 应用程序配置
//   - rt: 运行时实例，包含事件总线和检查器
//   - mgr: 提供者管理器
func runDemoInputPipeline(cfg *config.Config, rt *runtime.Runtime, mgr *providers.Manager) {
	sessionID := "demo-session"
	traceID := "demo-trace"

	// 构建语音管道
	pipe := bootstrap.BuildVoicePipeline(cfg, mgr)
	// 创建模拟音频帧源，使用默认演示帧数据
	source := audio.NewMockFrameSource(audio.DefaultDemoFrames(), 40*time.Millisecond)

	// 在后台运行管道
	go func() {
		if err := pipe.Run(
			context.Background(),
			source,
			// VAD语音开始回调
			func() {
				rt.Bus.Publish(bus.Event{Name: "audio.vad.start", SessionID: sessionID, TraceID: traceID})
				log.Printf("[demo] vad.speech_start")
			},
			// VAD语音结束回调
			func() {
				rt.Bus.Publish(bus.Event{Name: "audio.vad.stop", SessionID: sessionID, TraceID: traceID})
			},
			// ASR部分结果回调
			func(partial string) {
				rt.Bus.Publish(bus.Event{Name: "asr.partial", SessionID: sessionID, TraceID: traceID, Payload: partial})
			},
			// ASR最终结果回调
			func(final string) {
				rt.Bus.Publish(bus.Event{Name: "asr.final", SessionID: sessionID, TraceID: traceID, Payload: final})
			},
		); err != nil {
			log.Printf("[demo] audio pipeline failed: %v", err)
		}
	}()
}
