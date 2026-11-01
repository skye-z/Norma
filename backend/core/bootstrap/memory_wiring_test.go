package bootstrap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	audioTts "norma/backend/core/audio/tts"
	"norma/backend/core/db"
	"norma/backend/core/memory"
	"norma/backend/core/providers"
	config "norma/backend/utils"
)

func memoryTestConfig(dir string) *config.Config {
	cfg := &config.Config{}
	cfg.RAG.Provider = "hybrid"
	cfg.RAG.VectorIndexPath = filepath.Join(dir, "vec.json")
	cfg.Memory.Dir = filepath.Join(dir, "memory")
	cfg.Memory.MarkdownPath = filepath.Join(dir, "memory.md") // 旧版单文件，用于迁移
	return cfg
}

// 记住一条事实后能被检索到；重新构建检索器（模拟重启）后仍然存在
func TestRememberedFactSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := memoryTestConfig(dir)
	mgr := providers.NewManager(filepath.Join(dir, "providers"))

	engine, err := db.NewEngine(filepath.Join(dir, "m.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	retriever, md := chooseRetriever(cfg, engine, mgr)
	if md == nil {
		t.Fatal("markdown memory is nil")
	}
	if err := md.Remember(context.Background(), "我的猫叫团子"); err != nil {
		t.Fatal(err)
	}
	if !searchFinds(t, retriever, "猫叫团子", "用户的猫叫团子") {
		t.Fatal("fact not retrievable right after remember")
	}

	// 新的数据库文件模拟索引丢失，只有记忆目录里的 MEMORY.md 保留
	engine2, err := db.NewEngine(filepath.Join(dir, "m2.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	retriever2, _ := chooseRetriever(cfg, engine2, mgr)
	if !searchFinds(t, retriever2, "猫叫团子", "用户的猫叫团子") {
		t.Fatal("fact lost after restart; index was not rebuilt from MEMORY.md")
	}
}

func TestMarkdownProviderReturnsWholeFile(t *testing.T) {
	dir := t.TempDir()
	cfg := memoryTestConfig(dir)
	cfg.RAG.Provider = "markdown"
	retriever, md := chooseRetriever(cfg, nil, providers.NewManager(filepath.Join(dir, "providers")))
	if err := md.Remember(context.Background(), "喜欢喝美式"); err != nil {
		t.Fatal(err)
	}
	if !searchFinds(t, retriever, "任何问题", "喜欢喝美式") {
		t.Fatal("markdown provider did not return file content")
	}
}

func searchFinds(t *testing.T, r memory.Retriever, q, want string) bool {
	t.Helper()
	chunks, err := r.Search(context.Background(), q, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chunks {
		if strings.Contains(c.Content, want) {
			return true
		}
	}
	return false
}

// Genie 的程序或模型缺失时必须退回其他合成器，应用不能因此失声或起不来
func TestGenieMissingFallsBackToAnotherTTS(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.TTS.Provider = "genie"
	cfg.TTS.Genie.Worker = filepath.Join(dir, "no-such-worker")
	cfg.TTS.Genie.DataDir = filepath.Join(dir, "no-data")
	cfg.TTS.Genie.CharacterDir = filepath.Join(dir, "no-char")
	got := chooseTTS(cfg, providers.NewManager(filepath.Join(dir, "providers")))
	if _, isGenie := got.(*audioTts.GenieTTS); isGenie {
		t.Fatal("genie must not be selected when files are missing")
	}
	if got == nil {
		t.Fatal("no fallback TTS returned")
	}
}

// 前端按 wake_status.keyword 读取唤醒词；字段名一旦变回 Keyword，气泡就会显示兜底的问号
func TestWakeStatusJSONUsesSnakeCaseKeysTheFrontendReads(t *testing.T) {
	raw, err := json.Marshal(WakeConfigStatus{Enabled: true, EffectiveMode: "wake_word_vad", Keyword: "诺玛", Ready: true})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"enabled", "effective_mode", "provider", "keyword", "ready", "reason", "model_dir", "keywords_file"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing JSON key %q in %s", key, raw)
		}
	}
	if m["keyword"] != "诺玛" {
		t.Errorf("keyword = %v", m["keyword"])
	}
}

// 画像（个人信息）每轮整体注入提示词，不应该再出现在检索结果里；长期记忆则相反
func TestProfileFactsAreInjectedNotRetrieved(t *testing.T) {
	dir := t.TempDir()
	cfg := memoryTestConfig(dir)
	engine, err := db.NewEngine(filepath.Join(dir, "m.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	retriever, svc := chooseRetriever(cfg, engine, providers.NewManager(filepath.Join(dir, "providers")))
	ctx := context.Background()
	if err := svc.Remember(ctx, "我叫 Skye"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remember(ctx, "我的猫叫团子"); err != nil {
		t.Fatal(err)
	}
	if got := svc.Profile(); got != "- 用户叫 Skye" {
		t.Fatalf("profile = %q", got)
	}
	if searchFinds(t, retriever, "我叫什么名字", "用户叫 Skye") {
		t.Fatal("profile entries must not be duplicated into retrieval results")
	}
	if !searchFinds(t, retriever, "猫叫什么", "用户的猫叫团子") {
		t.Fatal("long-term fact must be retrievable")
	}
}

// 旧版单文件记忆在启动时自动迁移进新的记忆目录
func TestLegacyMemoryFileIsMigratedOnStartup(t *testing.T) {
	dir := t.TempDir()
	cfg := memoryTestConfig(dir)
	if err := os.WriteFile(cfg.Memory.MarkdownPath, []byte("- 我叫 Skye\n- 明天下午开会\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, err := db.NewEngine(filepath.Join(dir, "m.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	retriever, svc := chooseRetriever(cfg, engine, providers.NewManager(filepath.Join(dir, "providers")))
	if svc.Profile() != "- 用户叫 Skye" {
		t.Fatalf("profile after migration = %q", svc.Profile())
	}
	if !searchFinds(t, retriever, "开会", "明天下午开会") {
		t.Fatal("migrated long-term entry not retrievable")
	}
	if _, err := os.Stat(cfg.Memory.MarkdownPath + ".migrated"); err != nil {
		t.Fatal("legacy file should be kept as .migrated")
	}
}

// 没有配置大模型接口、或关闭开关时，不创建后台整理，也不能崩
func TestCuratorIsOnlyCreatedWhenEnabledAndConfigured(t *testing.T) {
	svc := memory.NewMemoryService(memory.NewMemoryBank(t.TempDir(), 0, 0))
	cfg := &config.Config{}
	cfg.Memory.AutoCurate = true
	if newCurator(cfg, svc) != nil {
		t.Fatal("no llm provider configured: curator must not be created")
	}
	cfg.LLM.Provider = "deepseek"
	if newCurator(cfg, svc) == nil {
		t.Fatal("deepseek has default model/endpoint: curator expected")
	}
	cfg.Memory.AutoCurate = false
	if newCurator(cfg, svc) != nil {
		t.Fatal("switch off: curator must not be created")
	}
	cfg.Memory.AutoCurate = true
	cfg.LLM.Provider = "ollama" // 没有模型名和地址
	if newCurator(cfg, svc) != nil {
		t.Fatal("ollama without model/endpoint: curator must not be created")
	}
	if newCurator(cfg, nil) != nil {
		t.Fatal("nil service must be tolerated")
	}
}

// 控制台里的“召回条数”必须真的传给编排器；此前传的是写死的 4
func TestRetrievalTopKFromConfigReachesTheOrchestrator(t *testing.T) {
	dir := t.TempDir()
	cfg := memoryTestConfig(dir)
	cfg.RAG.TopK = 3
	cfg.TTS.Provider = "system"
	cfg.LLM.Provider = "mock"
	engine, err := db.NewEngine(filepath.Join(dir, "m.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	st, err := NewStack(cfg, engine)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Orchestrator.RagTopK(); got != 3 {
		t.Fatalf("orchestrator top-k = %d, want 3 from config", got)
	}
}
