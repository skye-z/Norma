# Norma

> 一个基于 Go、Wails 和 Live2D 的桌面 AI 伴侣项目。

## 技术栈

- 桌面框架：Wails
- 前端渲染：PIXI + pixi-live2d-display
- 唤醒词：Sherpa ONNX
- 语音识别：Whisper.cpp
- 语音合成：Sherpa ONNX Kokoro
- 大模型 / Embedding：OpenAI 兼容接口
- 本地数据：SQLite + 本地资源目录

## 当前语音链路

当前项目的主要语音链路如下：

`mic -> wake -> vad/endpoint -> asr -> rag -> llm -> tts -> audio play -> live2d`

目前已完成的关键调整：

- 桌面伴侣页面会根据配置决定是否显示麦克风按钮。
- 自动唤醒模式下，先进入 wake 链路，命中后再进入 ASR。
- 唤醒前的语音会保留一段缓冲，命中后带着缓冲音频一起送进 ASR。
- 浏览器端上传原始采样率，后端统一重采样到 `16k`。
- 前端默认优先使用 `AudioWorklet`，不支持时回退到 `ScriptProcessor`。
- 麦克风上行优先走 WebSocket。
- wake / endpoint 的时序按约 `20ms` 帧粒度对齐。

## 唤醒词

唤醒资源目录：

- `resource/runtime/providers/wake/current/`

当前项目会在启动自检时检查 wake 资源：

- 如果 `encoder / decoder / joiner / tokens.txt` 存在但 `keywords.txt` 缺失，会自动创建。
- 默认唤醒词是 `诺玛`。
- `keywords.txt` 会写成 Sherpa 需要的关键词格式，而不是直接写中文。
- 在 console 修改唤醒词时，也会同步写回 `keywords.txt`。

## ASR

当前 ASR 默认优先使用 Whisper 嵌入模型，推荐模型顺序如下：

- `ggml-large-v3-turbo-q5_0.bin`
- `ggml-large-v3.bin`
- `ggml-medium.bin`
- `ggml-small.bin`
- `ggml-base.bin`

为了减少截断和误识别，当前链路还做了这些处理：

- 默认 `endpoint_silence_frames=20`
- 结束时会主动 flush 最后一段 utterance，避免尾句丢失。
- ASR 输出进入 LLM 前，会做一层轻量中文后处理纠偏。
- Whisper 输入目前保持原始语音样本进入模型。

## TTS

当前 TTS 使用 Sherpa ONNX Kokoro：

- 支持运行时切换声线。
- `sentence_first_enabled` 控制是否按句分段播报。
- TTS 输入会先做字符清洗，尽量避免 `unknown token`。
- console 可配置 `voice`、`speed`，语速支持 `1 / 1.25 / 1.5 / 1.75 / 2`。

## Console

当前 console 已支持以下核心配置：

- OpenAI 兼容 `endpoint`、`API Key`、`model`
- `persona`
- 对话测试按钮，可直接向 LLM 发送测试请求
- 输出模块中的 `voice`、`speed`、`sentence_first_enabled`

此外，LLM 回复现在支持少量插入 Live2D 控制标识，例如：

- `{{thinking}}`
- `{{smile}}`
- `{{motion:thinking}}`
- `{{expression:smile}}`

这些标识只用于驱动 Live2D，不会进入 TTS 正文。

## Windows 运行说明

Windows 下需要确保 Sherpa runtime DLL 可被正确加载：

- 启动脚本会设置 `SHERPA_DLL_DIR`
- `wails dev` / build 后会复制 sherpa 相关 DLL 到 `build/bin`
- 这样可以避免旧的 `onnxruntime.dll` 抢占导致 API version 不匹配

## 启动

使用开发脚本启动：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/wails-dev-embedded.ps1
```

UTF-8 检查：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/check-utf8.ps1 -Root .
powershell -ExecutionPolicy Bypass -File scripts/preflight.ps1 -Root .
```
