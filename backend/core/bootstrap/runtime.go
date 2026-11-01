// Package bootstrap 提供运行时启动和组件初始化功能
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

// 运行时组件栈
type Stack struct {
	Runtime         *runtime.Runtime
	Tracker         *config.Tracker
	ProviderManager *providers.Manager
	Retriever       memory.Retriever
	Memory          *memory.MarkdownMemory
	LLM             llm.Streamer
	TTS             audio.TTS
	Playback        audioPlayback
}

// 唤醒词配置状态
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

// audioPlayback 音频播放接口
type audioPlayback interface {
	Queue(sessionID, traceID string, chunk []byte) error
	Complete(sessionID, traceID string)
	Stop(sessionID, traceID, reason string)
	Drain(ctx context.Context, sessionID string) error
	Wait(ctx context.Context, sessionID string) error
}

// 创建运行时栈
func NewStack(cfg *config.Config, engine *xorm.Engine) (*Stack, error) {
	providerMgr := providers.NewManager(layout.ProvidersDir)
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
	retriever, mdMemory := chooseRetriever(cfg, engine, providerMgr)
	orch := runtime.NewOrchestratorWithPlayback(rt, retriever, streamer, tts, 4, playback)
	if mdMemory != nil {
		orch.SetMemory(mdMemory)
	}

	return &Stack{
		Runtime:         rt,
		Tracker:         tracker,
		ProviderManager: providerMgr,
		Retriever:       retriever,
		Memory:          mdMemory,
		LLM:             streamer,
		TTS:             tts,
		Playback:        playback,
	}, nil
}

// 构建语音管道
func BuildVoicePipeline(cfg *config.Config, mgr *providers.Manager) audio.Pipeline {
	return BuildVoicePipelineForMode(cfg, mgr, "")
}

// 构建指定模式的语音管道
func BuildVoicePipelineForMode(cfg *config.Config, mgr *providers.Manager, overrideMode string) audio.Pipeline {
	wakeStatus := ResolveWakeConfig(cfg, mgr)
	effectiveMode := wakeStatus.EffectiveMode
	if strings.TrimSpace(overrideMode) != "" {
		effectiveMode = strings.TrimSpace(overrideMode)
	}
	wakeBufferFrames := cfg.Audio.WakeBufferFrames
	if wakeBufferFrames <= 0 {
		wakeBufferFrames = 10
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

// 解析唤醒词配置
func ResolveWakeConfig(cfg *config.Config, mgr *providers.Manager) WakeConfigStatus {
	status := WakeConfigStatus{
		Enabled:       cfg != nil && cfg.Audio.EnableWakeWord,
		EffectiveMode: audio.WakeModeVADOnly,
		Provider:      "sherpa-onnx",
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
	// 查找模型文件目录和onnx模型文件
	modelDir := ""
	if mgr != nil {
		modelDir = filepath.Join(mgr.Root(), "wake", "current")
	}
	if custom := strings.TrimSpace(cfg.Audio.WakeModelPath); custom != "" {
		modelDir = custom
		if mgr != nil {
			mgr = providers.NewManager(filepath.Dir(custom))
		}
	}
	status.ModelDir = modelDir
	encoder := ""
	decoder := ""
	joiner := ""
	tokens := ""
	keywords := ""
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

// choosePlayback 选择音频播放实现
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

// chooseWakeWord 选择唤醒词检测模块
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

// choosePreprocessors 选择音频预处理模块
func choosePreprocessors(cfg *config.Config) []audio.Preprocessor {
	if cfg.Audio.AEC == "" && cfg.Audio.ANS == "" {
		return nil
	}
	label := "speaker"
	return []audio.Preprocessor{audio.NewPassthroughAECNS(label)}
}

// chooseASR 选择ASR模块
func chooseASR(cfg *config.Config, mgr *providers.Manager) audio.ASR {
	switch strings.ToLower(strings.TrimSpace(cfg.ASR.Provider)) {
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

// chooseTTS 选择TTS模块
func chooseTTS(cfg *config.Config, mgr *providers.Manager) audio.TTS {
	fallback := &audio.UnavailableTTS{Reason: "tts unavailable"}
	switch strings.ToLower(strings.TrimSpace(cfg.TTS.Provider)) {
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
		speed := float32(cfg.TTS.Speed)
		if speed <= 0 {
			speed = 1.25
		}
		primary := audioTts.NewSherpaKokoroTTS(modelDir, cfg.TTS.Voice, sid, speed, 2)
		log.Printf("[tts] use sherpa kokoro model_dir=%s sid=%d voice=%s speed=%.2f", modelDir, sid, cfg.TTS.Voice, speed)
		return primary
	case "genie":
		return newGenieTTS(cfg, fallback)
	case "system", "say", "macos":
		voice := strings.TrimSpace(cfg.TTS.Voice)
		rate := 0
		if cfg.TTS.Speed > 0 {
			rate = int(180 * cfg.TTS.Speed)
		}
		log.Printf("[tts] use macOS say voice=%s rate=%d", voice, rate)
		return audioTts.NewSystemSayTTS(voice, rate)
	default:
		return &audio.UnavailableTTS{Reason: "unsupported tts provider: " + strings.TrimSpace(cfg.TTS.Provider)}
	}
}

// chooseLLMStreamer 选择LLM流式处理器
func chooseLLMStreamer(cfg *config.Config) llm.Streamer {
	fallback := llm.UnavailableStreamer{Reason: "llm unavailable"}
	provider := strings.ToLower(strings.TrimSpace(cfg.LLM.Provider))
	if provider != "ollama" && provider != "openai" && provider != "deepseek" {
		return fallback
	}
	if provider == "deepseek" {
		model := strings.TrimSpace(cfg.LLM.Model)
		if model == "" {
			model = "deepseek-flash"
		}
		endpoint := strings.TrimSpace(cfg.LLM.Endpoint)
		if endpoint == "" {
			endpoint = "https://api.deepseek.com"
		}
		primary := llm.NewOllamaStreamer(endpoint, model, cfg.LLM.APIKey, 30*time.Second).WithOptions(true, cfg.LLM.MaxTokens, cfg.LLM.Temperature)
		log.Printf("[llm] use deepseek endpoint=%s model=%s thinking=off", endpoint, model)
		return llm.FallbackStreamer{Primary: primary, Fallback: fallback}
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
	primary := llm.NewOllamaStreamer(cfg.LLM.Endpoint, model, cfg.LLM.APIKey, 15*time.Second)
	degradedModel := ""
	if degradedModel == "" || degradedModel == model {
		log.Printf("[llm] use ollama provider endpoint=%s model=%s", cfg.LLM.Endpoint, model)
		return llm.FallbackStreamer{Primary: primary, Fallback: fallback}
	}

	log.Printf("[llm] use ollama provider endpoint=%s primary=%s degraded=%s", cfg.LLM.Endpoint, model, degradedModel)
	return llm.FallbackStreamer{Primary: primary, Fallback: fallback}
}

// chooseRetriever 选择记忆检索器，同时返回 Markdown 记忆服务用于写入“记住……”的内容
// rag.provider 为 markdown 时整份文件直接作为上下文；否则对条目建立全文与向量索引做混合检索
func chooseRetriever(cfg *config.Config, engine *xorm.Engine, mgr *providers.Manager) (memory.Retriever, *memory.MarkdownMemory) {
	mdPath := strings.TrimSpace(cfg.Memory.MarkdownPath)
	if mdPath == "" {
		mdPath = layout.MemoryPath
	}
	store := memory.NewMarkdownStore(mdPath, cfg.Memory.MarkdownMaxBytes)

	if strings.EqualFold(strings.TrimSpace(cfg.RAG.Provider), "markdown") {
		log.Printf("[rag] use markdown retriever file=%s", mdPath)
		return memory.MarkdownRetriever{Store: store}, memory.NewMarkdownMemory(store)
	}

	// 创建基于SQLite的全文搜索存储
	fts := memory.NewSQLiteStore(engine)
	syncMarkdown := func(indexes ...*memory.DiskVectorStore) *memory.MarkdownMemory {
		var md *memory.MarkdownMemory
		if len(indexes) > 0 && indexes[0] != nil {
			md = memory.NewMarkdownMemory(store, fts, indexes[0])
		} else {
			md = memory.NewMarkdownMemory(store, fts)
		}
		if err := md.Sync(); err != nil {
			log.Printf("[rag] sync markdown memory failed: %v", err)
		}
		return md
	}

	vecPath := strings.TrimSpace(cfg.RAG.VectorIndexPath)
	if vecPath == "" {
		vecPath = layout.VectorIndexPath
	}
	// 确保向量索引目录存在
	if dir := filepath.Dir(vecPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("[rag] create vector memory dir failed, fallback to text-only retriever: %v", err)
			return memory.NewHybridRetriever(nil, fts, nil), syncMarkdown()
		}
	}

	// 创建向量化嵌入器
	embedder := chooseEmbedder(cfg, mgr)
	vec, err := memory.NewDiskVectorStoreWithEmbedder(vecPath, embedder)
	if err != nil {
		log.Printf("[rag] init vector store failed, fallback to text-only retriever: %v", err)
		return memory.NewHybridRetriever(nil, fts, nil), syncMarkdown()
	}

	log.Printf("[rag] use hybrid retriever vector=%s embed_provider=%s markdown=%s", vecPath, cfg.RAG.EmbedProvider, mdPath)
	r := memory.NewHybridRetriever(vec, fts, nil)
	// 记忆条目彼此独立，相邻合并会把无关条目拼进同一段上下文
	r.SetPostProcessOptions(memory.RetrieverPostProcessOptions{EnableDedup: true, EnableRerank: true, RerankWeight: 0.15})
	return r, syncMarkdown(vec)
}

// chooseEmbedder 选择嵌入向量生成模块
func chooseEmbedder(cfg *config.Config, mgr *providers.Manager) memory.Embedder {
	_ = mgr
	switch strings.ToLower(strings.TrimSpace(cfg.RAG.EmbedProvider)) {
	case "ollama":
		if strings.TrimSpace(cfg.RAG.EmbedModel) == "" {
			log.Printf("[rag] ollama embed model empty, fallback to hash embedder")
			return memory.HashEmbedder{}
		}
		return memory.NewOllamaEmbedder(cfg.RAG.EmbedEndpoint, cfg.RAG.EmbedModel, 15*time.Second)
	case "static":
		e, err := memory.NewStaticEmbedder(strings.TrimSpace(cfg.RAG.EmbedModelDir))
		if err != nil {
			log.Printf("[rag] static embedder unavailable, fallback to hash embedder: %v", err)
			return memory.HashEmbedder{}
		}
		return e
	default:
		return memory.HashEmbedder{}
	}
}

// 确保唤醒词关键词文件存在
func EnsureWakeKeywordsForConfig(cfg *config.Config, mgr *providers.Manager) (string, error) {
	if cfg == nil {
		return "", nil
	}
	keyword := strings.TrimSpace(cfg.Audio.WakeKeyword)
	if keyword == "" {
		keyword = "诺玛"
	}
	if mgr == nil {
		return "", nil
	}
	if custom := strings.TrimSpace(cfg.Audio.WakeModelPath); custom != "" {
		return providers.EnsureWakeKeywordsFile(custom, keyword)
	}
	return mgr.EnsureWakeKeywords(keyword)
}

// firstExistingPath 返回第一个存在的文件路径
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

// newGenieTTS 创建 Genie 合成器并在后台预热子进程，避免第一句话承担模型加载耗时
func newGenieTTS(cfg *config.Config, fallback audio.TTS) audio.TTS {
	g := cfg.TTS.Genie
	bin := strings.TrimSpace(cfg.TTS.BinaryPath)
	if bin == "" || strings.TrimSpace(g.DataDir) == "" || strings.TrimSpace(g.CharacterDir) == "" {
		log.Printf("[tts] genie needs tts.binary_path, tts.genie.data_dir and tts.genie.character_dir")
		return fallback
	}
	command := []string{bin}
	if script := strings.TrimSpace(g.Script); script != "" {
		command = append(command, script)
	}
	refs := map[string]audioTts.GenieRef{}
	for name, r := range g.References {
		refs[strings.ToLower(strings.TrimSpace(name))] = audioTts.GenieRef{Audio: r.Audio, Text: r.Text}
	}
	if _, ok := refs["default"]; !ok {
		def, err := audioTts.DefaultGenieRef(g.CharacterDir)
		if err != nil {
			log.Printf("[tts] genie has no default reference audio: %v", err)
			return fallback
		}
		refs["default"] = def
	}
	tts := audioTts.NewGenieTTS(audioTts.GenieConfig{
		Command: command, DataDir: g.DataDir, CharacterDir: g.CharacterDir, Language: g.Language, Refs: refs,
	})
	go func() {
		if err := tts.Warmup(); err != nil {
			log.Printf("[tts] genie warmup failed: %v", err)
		}
	}()
	log.Printf("[tts] use genie worker=%s character=%s refs=%d", bin, g.CharacterDir, len(refs))
	return tts
}
