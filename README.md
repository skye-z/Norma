# Norma

> 基于 Go、Wails 与 Live2D 的桌面 AI 伴侣应用。

## 当前形态

- 桌面壳：Wails
- 渲染层：PIXI + pixi-live2d-display
- 唤醒：Sherpa ONNX 内嵌 wake word
- ASR：Whisper.cpp 内嵌识别
- TTS：Sherpa ONNX Kokoro
- LLM / Embedding：OpenAI 兼容接口
- 数据：SQLite + 本地资源目录

## 语音链路

当前语音链路已经统一为内嵌方案：

`mic -> wake -> vad/endpoint -> asr -> rag -> llm -> tts -> audio play -> live2d`

其中：

- 自动唤醒模式下，Live2D 加载完成后会自动进入后台待机会话
- 唤醒前音频会先进入 wake 检测，不会直接进入 ASR
- 唤醒命中后会携带唤醒词附近的缓存帧进入 ASR，尽量保留句首
- 手动模式下点击麦克风会直接进入识别，不要求先喊唤醒词
- 音频采集优先使用 `AudioWorklet`，不支持时回退到 `ScriptProcessor`
- 麦克风上行优先使用 WebSocket，停止时会先 flush 剩余帧再发送 EOF，避免句尾丢失

## 唤醒词

唤醒模型目录默认位于：

- `resource/runtime/providers/wake/current/`

启动自检会自动检查 wake 资源：

- 若 `encoder / decoder / joiner / tokens.txt` 存在但 `keywords.txt` 缺失，会自动创建
- 默认唤醒词为 `诺玛`
- `keywords.txt` 会写成 Sherpa 可识别格式，而不是直接写原始中文
- 在 console 中修改唤醒词时，会同步回写 `keywords.txt`

## ASR

当前 ASR 默认使用嵌入式 Whisper，并优先选择更高质量模型：

- `ggml-large-v3-turbo-q5_0.bin`
- `ggml-large-v3.bin`
- `ggml-medium.bin`
- `ggml-small.bin`
- `ggml-base.bin`

另外已做的修正包括：

- 句尾静音阈值默认提升到 `endpoint_silence_frames=20`
- 流结束时会强制 flush 尚未完成的 utterance，减少句尾被截断
- ASR 输出进入 LLM 前会做一层中文后处理，优先修正常见的“诺玛”近音误识别

## TTS

当前 TTS 使用 Sherpa ONNX Kokoro：

- 声线与语速支持运行时热更新
- TTS 输入会先清洗异常字符与危险标点，减少 `unknown token` 类错误
- console 支持配置 `voice`、`speed`，语速当前提供 `1 / 1.25 / 1.5 / 1.75 / 2`

## Windows 运行时

Windows 下已增加 Sherpa runtime DLL 处理：

- 启动脚本会准备 `SHERPA_DLL_DIR`
- `wails dev` / build 后会把 sherpa 相关 DLL 复制到 `build/bin`
- 避免系统里旧版 `onnxruntime.dll` 抢占导致 API version 不匹配

## 运行

开发启动：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/wails-dev-embedded.ps1
```

UTF-8 与环境预检：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/check-utf8.ps1 -Root .
powershell -ExecutionPolicy Bypass -File scripts/preflight.ps1 -Root .
```

## 关键路径

- Wake：`resource/runtime/providers/wake/current/`
- ASR：`resource/runtime/providers/asr/current/`
- TTS：`resource/runtime/providers/tts/current/`
- Live2D：`resource/model/`
- 控制台：`frontend/public/console/`

## 日志观察点

排查语音链路时，优先关注这些日志：

- `[wake] embedded sherpa ready ...`
- `[conversation] pipeline init ...`
- `[pipeline] wake detected`
- `[pipeline] flush utterance reason=end_of_stream ...`
- `[asr] postprocess raw=... normalized=...`
- `[tts] kokoro sanitized input raw=... sanitized=...`
- `[audio] capture backend=audioworklet`

## 更多文档

- 使用说明：`USE.md`
- 文档站：`docs/content/docs/usage/`
- 开发文档：`docs/content/docs/development/`
