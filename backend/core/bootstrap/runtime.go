// Package bootstrap 提供运行时启动和组件初始化的核心功能
// 负责构建完整的语音交互系统，包括音频处理、LLM、记忆检索等组件
package bootstrap

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"norma/backend/core/audio"
	audioAsr "norma/backend/core/audio/asr"
	audioTts "norma/backend/core/audio/tts"
	audioVad "norma/backend/core/audio/vad"
	audioWake "norma/backend/core/audio/wake"
	"norma/backend/core/live2d"
	"norma/backend/core/llm"
	"norma/backend/core/memory"
	"norma/backend/core/providers"
	"norma/backend/core/runtime"
	"norma/backend/core/runtime/bus"
	"norma/backend/utils"
	"norma/pkg/layout"
	"xorm.io/xorm"
)

// Stack 包含运行时所需的所有核心组件
// 构建整个应用的服务栈，包括运行时、追踪器、提供商管理器、检索器、LLM、TTS和音频播放
type Stack struct {
	Runtime         *runtime.Runtime
	Tracker         *config.Tracker
	ProviderManager *providers.Manager
	Retriever       memory.Retriever
	LLM             llm.Streamer
	TTS             audio.TTS
	Playback        audioPlayback
}

// WakeConfigStatus 描述唤醒词配置的状态信息
// 包含启用状态、实际模式、提供商、关键词、就绪状态和原因说明
type WakeConfigStatus struct {
	Enabled       bool
	EffectiveMode string
	Provider      string
	Keyword       string
	Ready         bool
	Reason        string
	ModelDir      string
	KeywordsFile  string
}

// audioPlayback 定义音频播放的接口
// 用于将TTS生成的音频数据排入队列、播放完成、停止播放、排空缓冲区和等待播放完成
type audioPlayback interface {
	Queue(sessionID, traceID string, chunk []byte) error
	Complete(sessionID, traceID string)
	Stop(sessionID, traceID, reason string)
	Drain(ctx context.Context, sessionID string) error
	Wait(ctx context.Context, sessionID string) error
}

// NewStack 创建并初始化完整的运行时栈
// 1. 初始化提供商管理器并确保布局文件存在
// 2. 为配置中的唤醒词关键词创建文件（如不存在）
// 3. 创建运行时实例并附加延迟观察器
// 4. 创建Live2D桥接器用于动画渲染
// 5. 选择并初始化TTS、音频播放、LLM流式处理器和检索器
// 6. 创建编排器协调所有组件
func NewStack(cfg *config.Config, engine *xorm.Engine) (*Stack, error) {
	providerMgr := providers.NewManager(cfg.Providers.Root)
	if err := providerMgr.EnsureLayout(); err != nil {
		return nil, err
	}
	if _, err := EnsureWakeKeywordsForConfig(cfg, providerMgr); err != nil {
		log.Printf("[wake] ensure keywords failed: %v", err)
	}
	log.Printf("[build] embedded whisper: %v", audio.EmbeddedWhisperEnabled())
	log.Printf("[build] embedded sherpa: %v", audio.EmbeddedSherpaEnabled())

	rt := runtime.New()
	tracker := config.NewTracker()
	runtime.AttachLatencyObserver(rt, tracker)

	_ = live2d.NewBridge(rt.Bus)

	tts := chooseTTS(cfg, providerMgr)
	playback := choosePlayback(rt.Bus, tts)
	streamer := chooseLLMStreamer(cfg)
	retriever := chooseRetriever(cfg, engine, providerMgr)
	_ = runtime.NewOrchestratorWithPlayback(rt, retriever, streamer, tts, cfg.RAG.TopK, playback)

	return &Stack{
		Runtime:         rt,
		Tracker:         tracker,
		ProviderManager: providerMgr,
		Retriever:       retriever,
		LLM:             streamer,
		TTS:             tts,
		Playback:        playback,
	}, nil
}

// BuildVoicePipeline 构建默认的语音处理管道
// 使用配置中的设置创建完整的语音交互管道
func BuildVoicePipeline(cfg *config.Config, mgr *providers.Manager) audio.Pipeline {
	return BuildVoicePipelineForMode(cfg, mgr, "")
}

// BuildVoicePipelineForMode 构建指定模式的语音处理管道
// 支持覆盖唤醒模式，用于不同的语音交互场景
func BuildVoicePipelineForMode(cfg *config.Config, mgr *providers.Manager, overrideMode string) audio.Pipeline {
	wakeStatus := ResolveWakeConfig(cfg, mgr)
	effectiveMode := wakeStatus.EffectiveMode
	if strings.TrimSpace(overrideMode) != "" {
		effectiveMode = strings.TrimSpace(overrideMode)
	}
	// 计算唤醒窗口的帧数，每20ms一帧
	wakeBufferFrames := cfg.Audio.WakeWindowMs / 20
	if cfg.Audio.WakeWindowMs%20 != 0 {
		wakeBufferFrames++
	}
	if wakeBufferFrames <= 0 {
		wakeBufferFrames = 1
	}
	pipe := audio.Pipeline{
		VAD:              audioVad.NewEnergyVAD(0.02),
		WakeWord:         chooseWakeWord(cfg, mgr),
		ASR:              chooseASR(cfg, mgr),
		WakeMode:         effectiveMode,
		WakeBufferFrames: wakeBufferFrames,
		SilenceFrames:    cfg.Audio.EndpointSilenceFrames,
		AllowBargeIn:     cfg.Audio.InterruptDuringSpeak,
	}
	// 如果不是唤醒词+VAD模式，或唤醒功能未启用，则禁用唤醒词检测
	if effectiveMode != audio.WakeModeWakeWordVAD || !wakeStatus.Enabled || !wakeStatus.Ready {
		pipe.WakeWord = nil
	}
	pipe.Preprocessors = choosePreprocessors(cfg)
	return pipe
}

// ResolveWakeConfig 解析并验证唤醒词配置
// 检查配置有效性、模式支持、提供商可用性和模型文件完整性
// 返回详细的配置状态，包括是否就绪及原因
func ResolveWakeConfig(cfg *config.Config, mgr *providers.Manager) WakeConfigStatus {
	status := WakeConfigStatus{
		Enabled:       cfg != nil && cfg.Audio.EnableWakeWord,
		EffectiveMode: audio.WakeModeVADOnly,
		Provider:      strings.ToLower(strings.TrimSpace(cfg.Audio.WakeProvider)),
		Keyword:       strings.TrimSpace(cfg.Audio.WakeKeyword),
	}
	if cfg == nil {
		status.Reason = "config unavailable"
		return status
	}
	if !cfg.Audio.EnableWakeWord {
		status.Reason = "audio.enable_wake_word=false"
		return status
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Audio.WakeMode))
	if mode == "" {
		mode = audio.WakeModeWakeWordVAD
	}
	if mode != audio.WakeModeWakeWordVAD {
		status.EffectiveMode = audio.WakeModeVADOnly
		status.Reason = "wake_mode is not wake_word_vad"
		return status
	}
	status.EffectiveMode = audio.WakeModeWakeWordVAD
	if status.Provider == "" {
		status.Reason = "wake provider is empty"
		return status
	}
	if status.Provider != "sherpa-onnx" {
		status.Reason = "unsupported wake provider: " + status.Provider
		return status
	}
	// 查找模型文件目录和onnx模型文件
	modelDir := strings.TrimSpace(cfg.Audio.WakeModelPath)
	if modelDir == "" && mgr != nil {
		modelDir = filepath.Join(mgr.Root(), "wake", "current")
	}
	status.ModelDir = modelDir
	encoder := ""
	decoder := ""
	joiner := ""
	tokens := ""
	keywords := strings.TrimSpace(cfg.Audio.WakeKeywordsFilePath)
	if mgr != nil {
		encoder = mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx")
		decoder = mgr.FindGlob("wake", "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx")
		joiner = mgr.FindGlob("wake", "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx")
		tokens = mgr.FindFirst("wake", "tokens.txt")
		if keywords == "" {
			keywords = mgr.FindFirst("wake", "keywords.txt", "test_keywords.txt")
		}
	}
	// 尝试在模型目录中查找onnx模型文件
	if encoder == "" {
		encoder = firstExistingPath(filepath.Join(modelDir, "encoder-epoch-*.onnx"), filepath.Join(modelDir, "encoder.int8.onnx"), filepath.Join(modelDir, "encoder.onnx"))
	}
	if decoder == "" {
		decoder = firstExistingPath(filepath.Join(modelDir, "decoder-epoch-*.onnx"), filepath.Join(modelDir, "decoder.int8.onnx"), filepath.Join(modelDir, "decoder.onnx"))
	}
	if joiner == "" {
		joiner = firstExistingPath(filepath.Join(modelDir, "joiner-epoch-*.onnx"), filepath.Join(modelDir, "joiner.int8.onnx"), filepath.Join(modelDir, "joiner.onnx"))
	}
	if tokens == "" {
		tokens = firstExistingPath(filepath.Join(modelDir, "tokens.txt"))
	}
	if keywords == "" {
		// 确保关键词文件存在
		if created, err := EnsureWakeKeywordsForConfig(cfg, mgr); err == nil && created != "" {
			keywords = created
		} else if err != nil {
			log.Printf("[wake] ensure keywords failed: %v", err)
		}
		if keywords == "" {
			keywords = firstExistingPath(filepath.Join(modelDir, "keywords.txt"), filepath.Join(modelDir, "test_keywords.txt"))
		}
	}
	status.KeywordsFile = keywords
	// 检查必要的模型文件是否完整
	missing := make([]string, 0, 5)
	if encoder == "" {
		missing = append(missing, "encoder")
	}
	if decoder == "" {
		missing = append(missing, "decoder")
	}
	if joiner == "" {
		missing = append(missing, "joiner")
	}
	if tokens == "" {
		missing = append(missing, "tokens")
	}
	if keywords == "" {
		missing = append(missing, "keywords")
	}
	if len(missing) > 0 {
		status.Reason = "wake assets incomplete: " + strings.Join(missing, ",")
		return status
	}
	status.Ready = true
	status.Reason = "ready"
	return status
}

// choosePlayback 根据TTS类型选择合适的音频播放实现
// 检查TTS是否提供音频格式信息，创建平台原生的播放接收器
func choosePlayback(b *bus.Bus, tts audio.TTS) audioPlayback {
	provider, ok := tts.(audio.AudioFormatProvider)
	if !ok {
		return nil
	}
	sink, err := audio.NewPlatformPlaybackSink(b, provider.SampleRate(), provider.Channels(), provider.Encoding())
	if err != nil {
		log.Printf("[audio] native playback unavailable: %v", err)
		return nil
	}
	return sink
}

// chooseWakeWord 根据配置选择唤醒词检测模块
// 支持sherpa-onnx嵌入式唤醒词检测，返回对应的唤醒词实例或不可用状态
func chooseWakeWord(cfg *config.Config, mgr *providers.Manager) audio.WakeWord {
	status := ResolveWakeConfig(cfg, mgr)
	if !status.Enabled || status.EffectiveMode != audio.WakeModeWakeWordVAD {
		return nil
	}
	provider := status.Provider
	if provider == "" {
		return &audio.UnavailableWakeWord{Reason: "wake provider is empty"}
	}
	if provider == "sherpa-onnx" {
		log.Printf("[wake] embedded sherpa enabled=%v", audio.EmbeddedSherpaEnabled())
		modelDir := status.ModelDir
		encoder := ""
		decoder := ""
		joiner := ""
		tokens := ""
		if mgr != nil {
			encoder = mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx")
			decoder = mgr.FindGlob("wake", "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx")
			joiner = mgr.FindGlob("wake", "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx")
			tokens = mgr.FindFirst("wake", "tokens.txt")
		}
		if encoder == "" {
			encoder = firstExistingPath(filepath.Join(modelDir, "encoder-epoch-*.onnx"), filepath.Join(modelDir, "encoder.int8.onnx"), filepath.Join(modelDir, "encoder.onnx"))
		}
		if decoder == "" {
			decoder = firstExistingPath(filepath.Join(modelDir, "decoder-epoch-*.onnx"), filepath.Join(modelDir, "decoder.int8.onnx"), filepath.Join(modelDir, "decoder.onnx"))
		}
		if joiner == "" {
			joiner = firstExistingPath(filepath.Join(modelDir, "joiner-epoch-*.onnx"), filepath.Join(modelDir, "joiner.int8.onnx"), filepath.Join(modelDir, "joiner.onnx"))
		}
		if tokens == "" {
			tokens = firstExistingPath(filepath.Join(modelDir, "tokens.txt"))
		}
		keywords := status.KeywordsFile
		if encoder == "" || decoder == "" || joiner == "" || tokens == "" || keywords == "" {
			log.Printf("[wake] sherpa-onnx assets incomplete encoder=%v decoder=%v joiner=%v tokens=%v keywords=%v", encoder != "", decoder != "", joiner != "", tokens != "", keywords != "")
			return &audio.UnavailableWakeWord{Reason: "sherpa-onnx assets incomplete"}
		}
		wake := audioWake.NewSherpaEmbeddedWakeWord(encoder, decoder, joiner, tokens, keywords, cfg.Audio.WakeKeyword)
		if _, unavailable := wake.(*audio.UnavailableWakeWord); unavailable {
			log.Printf("[wake] embedded sherpa constructor returned unavailable keyword=%s keywords=%s", cfg.Audio.WakeKeyword, keywords)
		} else {
			log.Printf("[wake] embedded sherpa ready keyword=%s keywords=%s", cfg.Audio.WakeKeyword, keywords)
		}
		return wake
	}
	log.Printf("[wake] unsupported provider=%s", provider)
	return &audio.UnavailableWakeWord{Reason: "unsupported wake provider: " + provider}
}

// choosePreprocessors 根据配置选择音频预处理模块
// 包括回声消除(AEC)和噪声抑制(NS)
func choosePreprocessors(cfg *config.Config) []audio.Preprocessor {
	if !cfg.Audio.AECEnabled && !cfg.Audio.NSEnabled {
		return nil
	}
	label := cfg.Audio.Scene
	if label == "" {
		label = "speaker"
	}
	return []audio.Preprocessor{audio.NewPassthroughAECNS(label)}
}

// chooseASR 根据配置选择自动语音识别(ASR)模块
// 支持HTTP接口、Whisper和Whisper.cpp等多种提供商
func chooseASR(cfg *config.Config, mgr *providers.Manager) audio.ASR {
	switch strings.ToLower(strings.TrimSpace(cfg.ASR.Provider)) {
	case "http":
		if cfg.ASR.Endpoint == "" {
			return audio.UnavailableASR{Reason: "asr.endpoint is empty"}
		}
		asr := audio.NewHTTPASR(cfg.ASR.Endpoint, time.Duration(cfg.ASR.RequestTimeoutMs)*time.Millisecond)
		asr.StreamEndpoint = strings.TrimSpace(cfg.ASR.StreamEndpoint)
		return asr
	case "whisper", "whispercpp":
		log.Printf("[asr] embedded whisper enabled=%v", audio.EmbeddedWhisperEnabled())
		model := mgr.ResolveFile("asr", "", cfg.ASR.ModelPath, "ggml-large-v3-turbo-q5_0.bin", "ggml-large-v3.bin", "ggml-medium.bin", "ggml-small.bin", "ggml-base.bin")
		if model == "" {
			log.Printf("[asr] whisper model not configured")
			return audio.UnavailableASR{Reason: "whisper model missing"}
		}
		// 针对中文使用base模型的建议提示
		if strings.EqualFold(strings.TrimSpace(cfg.ASR.Language), "zh") && strings.Contains(strings.ToLower(filepath.Base(model)), "ggml-base") {
			log.Printf("[asr] using base whisper model for zh; recommend ggml-small.bin or ggml-medium.bin for better accuracy")
		}
		// 优先尝试嵌入式Whisper
		if embedded, err := audioAsr.NewWhisperEmbeddedASR(model, cfg.ASR.Language, cfg.ASR.Threads); err == nil {
			log.Printf("[asr] use embedded whisper model=%s", model)
			return embedded
		} else {
			log.Printf("[asr] embedded whisper init failed: %v", err)
		}
		// 回退到whisper.cpp命令行工具
		binary := mgr.ResolveBinary("asr", "", cfg.ASR.BinaryPath, "whisper-cli.exe", "main.exe", "whisper-cli", "main")
		if binary == "" {
			log.Printf("[asr] whisper embedded unavailable and cli binary missing")
			return audio.UnavailableASR{Reason: "whisper embedded unavailable and cli binary missing"}
		}
		log.Printf("[asr] fallback whisper.cpp cli binary=%s model=%s", binary, model)
		return audioAsr.NewWhisperCPPASR(binary, model, cfg.ASR.Language, cfg.ASR.Threads)
	default:
		return audio.UnavailableASR{Reason: "unsupported asr provider: " + strings.TrimSpace(cfg.ASR.Provider)}
	}
}

// chooseTTS 根据配置选择文本转语音(TTS)模块
// 支持HTTP/XTTS、sherpa-onnx和kokoro等提供商
func chooseTTS(cfg *config.Config, mgr *providers.Manager) audio.TTS {
	fallback := &audio.UnavailableTTS{Reason: "tts unavailable"}
	switch strings.ToLower(strings.TrimSpace(cfg.TTS.Provider)) {
	case "http", "xtts":
		if cfg.TTS.Endpoint == "" {
			return fallback
		}
		return audio.NewXTTSStreamer(cfg.TTS.Endpoint, cfg.TTS.Voice, time.Duration(cfg.TTS.RequestTimeoutMs)*time.Millisecond)
	case "sherpa", "sherpa-onnx", "kokoro":
		modelPath := mgr.ResolveFile("tts", "", cfg.TTS.ModelPath, "model.onnx")
		if modelPath == "" {
			log.Printf("[tts] sherpa kokoro model not configured")
			return fallback
		}
		modelDir := filepath.Dir(modelPath)
		sid := -1
		if v := strings.TrimSpace(cfg.TTS.Voice); v != "" {
			sid = audioTts.ResolveSpeakerID(v)
		}
		if sid < 0 {
			sid = 45
		}
		speed := cfg.TTS.Speed
		if speed <= 0 {
			speed = 1.25
		}
		primary := audioTts.NewSherpaKokoroTTS(modelDir, cfg.TTS.Voice, sid, speed, 2)
		log.Printf("[tts] use sherpa kokoro model_dir=%s sid=%d voice=%s speed=%.2f", modelDir, sid, cfg.TTS.Voice, speed)
		return primary
	default:
		return &audio.UnavailableTTS{Reason: "unsupported tts provider: " + strings.TrimSpace(cfg.TTS.Provider)}
	}
}

// chooseLLMStreamer 根据配置选择LLM流式处理器
// 目前主要支持Ollama提供商，自动发现可用模型
func chooseLLMStreamer(cfg *config.Config) llm.Streamer {
	fallback := llm.UnavailableStreamer{Reason: "llm unavailable"}
	if cfg.LLM.Provider != "ollama" {
		return fallback
	}

	model := strings.TrimSpace(cfg.LLM.Model)
	if model == "" {
		// 自动发现第一个可用的LLM模型
		discovered, derr := llm.DiscoverFirstLLMModel(cfg.LLM.Endpoint, cfg.LLM.APIKey, 2*time.Second)
		if derr != nil {
			log.Printf("[llm] discover model failed: %v", derr)
			return fallback
		}
		model = discovered
	}
	primary := llm.NewOllamaStreamer(cfg.LLM.Endpoint, model, cfg.LLM.APIKey, time.Duration(cfg.LLM.RequestTimeoutMs)*time.Millisecond)
	degradedModel := ""
	if degradedModel == "" || degradedModel == model {
		log.Printf("[llm] use ollama provider endpoint=%s model=%s", cfg.LLM.Endpoint, model)
		return llm.FallbackStreamer{Primary: primary, Fallback: fallback}
	}

	log.Printf("[llm] use ollama provider endpoint=%s primary=%s degraded=%s", cfg.LLM.Endpoint, model, degradedModel)
	return llm.FallbackStreamer{Primary: primary, Fallback: fallback}
}

// chooseRetriever 根据配置选择记忆检索器
// 支持混合检索模式，结合向量检索和全文检索
func chooseRetriever(cfg *config.Config, engine *xorm.Engine, mgr *providers.Manager) memory.Retriever {
	// 创建基于SQLite的全文搜索存储
	fts := memory.NewSQLiteStore(engine)

	vecPath := strings.TrimSpace(cfg.RAG.VectorIndexPath)
	if vecPath == "" {
		vecPath = layout.VectorIndexPath
	}
	// 确保向量索引目录存在
	if dir := filepath.Dir(vecPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("[rag] create vector memory dir failed, fallback to text-only retriever: %v", err)
			return memory.NewHybridRetriever(nil, fts, nil)
		}
	}

	// 创建向量化嵌入器
	embedder := chooseEmbedder(cfg, mgr)
	vec, err := memory.NewDiskVectorStoreWithEmbedder(vecPath, embedder)
	if err != nil {
		log.Printf("[rag] init vector store failed, fallback to text-only retriever: %v", err)
		return memory.NewHybridRetriever(nil, fts, nil)
	}

	log.Printf("[rag] use hybrid retriever vector=%s embed_provider=%s", vecPath, cfg.RAG.EmbedProvider)
	return memory.NewHybridRetriever(vec, fts, nil)
}

// chooseEmbedder 根据配置选择嵌入向量生成模块
// 支持Ollama嵌入和简单的哈希嵌入
func chooseEmbedder(cfg *config.Config, mgr *providers.Manager) memory.Embedder {
	_ = mgr
	switch strings.ToLower(strings.TrimSpace(cfg.RAG.EmbedProvider)) {
	case "ollama":
		if strings.TrimSpace(cfg.RAG.EmbedModel) == "" {
			log.Printf("[rag] ollama embed model empty, fallback to hash embedder")
			return memory.HashEmbedder{}
		}
		return memory.NewOllamaEmbedder(cfg.RAG.EmbedEndpoint, cfg.RAG.EmbedModel, 15*time.Second)
	default:
		return memory.HashEmbedder{}
	}
}

// EnsureWakeKeywordsForConfig 确保唤醒词关键词文件存在
// 如果配置中指定了模型路径，则在指定目录创建关键词文件
// 否则使用提供商管理器处理
func EnsureWakeKeywordsForConfig(cfg *config.Config, mgr *providers.Manager) (string, error) {
	if cfg == nil {
		return "", nil
	}
	keyword := strings.TrimSpace(cfg.Audio.WakeKeyword)
	if keyword == "" {
		keyword = "诺玛"
	}
	modelDir := strings.TrimSpace(cfg.Audio.WakeModelPath)
	if modelDir != "" {
		return providers.EnsureWakeKeywordsFile(modelDir, keyword)
	}
	if mgr == nil {
		return "", nil
	}
	return mgr.EnsureWakeKeywords(keyword)
}

// firstExistingPath 返回第一个存在的文件路径
// 支持通配符匹配，按顺序尝试每个路径，返回第一个存在的
func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		// 处理通配符路径
		if strings.Contains(path, "*") {
			matches, _ := filepath.Glob(path)
			for _, match := range matches {
				if _, err := os.Stat(match); err == nil {
					return match
				}
			}
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
