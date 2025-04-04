# Norma - 诺玛

> 基于 Golang + Live2D 的本地优先桌面人格化伴侣。

## 核心目标

1. 轻量本地部署：双击启动，单机运行，无 Redis/ES 等中间件。
2. 高拟人化体验：Live2D 动作、情绪、口型联动。
3. 极简架构：Go 核心 + 事件总线 + 可替换能力层。

## 定版技术基线

- 后端：Golang
- 配置：Viper
- 数据层：XORM + SQLite（modernc.org/sqlite）
- 桌面层：Wails + Live2D
- Wake：Sherpa-onnx（内嵌）
- ASR：Whisper.cpp（内嵌）
- TTS：Piper
- LLM / Embedding：Ollama

## 架构总览

### 1) 分层架构

1. 交互层：Wails 窗口、Live2D 前端、mic 按钮、调试页。
2. 运行时层：状态机、事件总线、会话与取消机制。
3. 编排层：`asr.final -> rag.ready -> llm.token -> tts.chunk.ready -> audio.play.end`。
4. 能力层：语音、知识库检索、记忆、诊断与设置。
5. 基础层：SQLite、本地文件系统、Ollama、本地语音能力与模型资产。

### 2) 关键约束

1. 模块间通过事件通信，避免深度耦合。
2. 全链路携带 `session_id` / `trace_id`。
3. Speaking 阶段必须支持 interrupt 打断。
4. 默认本地优先，外网能力显式开关。

## 语音链路

目标链路：

`mic -> wake(sherpa embedded) -> vad/endpoint -> asr(whisper embedded) -> llm(ollama) -> tts(piper) -> audio play -> live2d`

关键事件：

`audio.vad.start / asr.partial / asr.final / rag.ready / llm.token / llm.done / tts.chunk.ready / audio.play.start / audio.play.end / interrupt`

## 知识库与索引架构

### 1) 数据来源

1. 本地文件（txt / md / pdf / docx）。
2. 对话与记忆衍生文本。

### 2) 索引技术

1. 关键词检索：SQLite FTS5（BM25）。
2. 向量检索：HNSW 本地向量索引。
3. 融合排序：RRF（Reciprocal Rank Fusion）。
4. 后处理：去重、相邻合并、轻量重排。

### 3) 检索流程

`query -> embedding(ollama) -> HNSW召回 + FTS5召回 -> RRF融合 -> 上下文整理 -> LLM`

### 4) 索引治理

1. 增量更新：监听新增/修改文件后重建受影响 chunk。
2. 回收删除：文件删除后清理对应索引项。
3. 元数据：保存路径、mtime、hash、chunk 溯源信息。

## 当前状态（简版）

已具备：

1. mic 会话 API、状态机/事件总线、基础编排模块，以及内嵌 Whisper / Sherpa 能力实现。
2. Hybrid RAG（FTS5 + HNSW + RRF）与本地记忆模块。
3. Live2D 事件驱动桥接（motion/mouth/emotion）。

仍需收口：

1. 桌面主流程中的编排与内嵌语音能力接线闭环。
2. 真实麦克风采集与真实播放链路稳定化。
3. 播放期口型同步与稳定 barge-in。

## 文档入口

- 项目计划：`TODO.md`
- 架构增强：`docs/架构增强方案.md`
- 语音设计：`docs/语音交互模块设计.md`
- 状态机与事件：`docs/状态机与事件总线.md`
- 测试与验收：`docs/测试与验收.md`
- 文档索引：`docs/README.md`
