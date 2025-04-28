# USE

## 简介

Norma 是一个带 Live2D 形象的桌面 AI 伴侣，当前核心能力包括：

- 唤醒词：`sherpa-onnx`
- 语音识别：`whisper.cpp`
- 语音合成：`Sherpa ONNX Kokoro`
- 大模型 / Embedding：OpenAI 兼容接口

## 启动

```powershell
powershell -ExecutionPolicy Bypass -File scripts/wails-dev-embedded.ps1
```

UTF-8 检查：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/check-utf8.ps1 -Root .
powershell -ExecutionPolicy Bypass -File scripts/preflight.ps1 -Root .
```

## 主要能力

### 自动唤醒

- Live2D 加载完成后，如果 wake ready 会自动进入后台待机会话
- 此时可能不显示麦克风按钮
- 命中唤醒词后才进入 ASR

### 手动模式

- 点击麦克风按钮即可直接进入识别
- 不需要先说唤醒词
- 会话结束后会根据策略恢复后台待机

### Console 配置

console 当前可配置：

- OpenAI 兼容 `endpoint`
- `API Key`
- `model`
- `persona`
- `voice`
- `speed`
- `sentence_first_enabled`
- LLM 测试按钮

## 唤醒词说明

- 默认唤醒词为 `诺玛`
- 关键词文件位于 `resource/runtime/providers/wake/current/keywords.txt`
- 如果 wake 模型存在但 `keywords.txt` 缺失，启动时会自动创建
- console 修改唤醒词后，会同步写回关键词文件

## ASR 说明

- 浏览器端上传原始采样率，后端统一重采样到 `16k`
- wake / endpoint 当前按约 `20ms` 帧粒度运行
- 结束时会做 final flush，减少尾句丢失
- 推荐优先使用 `ggml-large-v3-turbo-q5_0.bin`

## TTS 说明

- 当前使用 Kokoro 作为 TTS 引擎
- 支持运行时切换声线和语速
- TTS 输入会先清洗，避免 `unknown token`
- `sentence_first_enabled` 控制是否按句播放

## 前端音频链路

- 自动唤醒下优先使用 `AudioWorklet`
- 不支持时回退到 `ScriptProcessor`
- 麦克风上行优先使用 WebSocket

## 排障

### 唤醒词无效

1. 检查 wake 目录中是否存在 `encoder / decoder / joiner / tokens.txt`
2. 检查 `keywords.txt` 是否存在
3. 查看日志是否出现 `[wake] embedded sherpa ready`
4. 查看是否有 `[pipeline] wake detected`

### ASR 截断或误识别

1. 优先使用 `ggml-large-v3-turbo-q5_0.bin`
2. 检查日志是否出现 `[pipeline] flush utterance reason=end_of_stream ...`
3. 查看 `[asr] postprocess raw=... normalized=...`
4. 调整 `audio.endpoint_silence_frames`

### TTS / unknown token

1. 查看 `[tts] kokoro sanitized input ...`
2. 检查文本中是否包含异常字符
3. 确认当前 voice 和模型目录对应正确
