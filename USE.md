# USE

## 概览

Norma 当前是一套以内嵌语音链路为主的桌面伴侣应用：

- Wake：`sherpa-onnx`
- ASR：`whisper.cpp`
- TTS：`Sherpa ONNX Kokoro`
- LLM / Embedding：OpenAI 兼容接口

默认推荐直接使用项目内置的嵌入式运行方式，不再依赖旧的模块管理和自动降级页面。

## 启动

开发启动：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/wails-dev-embedded.ps1
```

UTF-8 与环境预检：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/check-utf8.ps1 -Root .
powershell -ExecutionPolicy Bypass -File scripts/preflight.ps1 -Root .
```

## 使用方式

### 自动唤醒

- Live2D 加载完成后，如果当前配置为自动唤醒且 wake ready，会自动进入后台待机
- 此时默认隐藏麦克风按钮
- 直接喊 `诺玛` 即可唤醒后续识别链路

### 手动唤醒

- 切换到手动模式后会显示麦克风按钮
- 点击麦克风后直接进入识别，不需要先喊唤醒词
- 结束后如果配置仍为自动唤醒，会重新恢复后台待机

### 中控台

console 对话模块当前支持：

- OpenAI 兼容 `endpoint`
- `API Key`
- `model`
- `persona`
- “测试”按钮，用于直接向 LLM 发送 `你好，请自我介绍一下`

## 唤醒词说明

- 默认唤醒词是 `诺玛`
- 唤醒词文件位于 `resource/runtime/providers/wake/current/keywords.txt`
- 如果 wake 模型存在而 `keywords.txt` 缺失，启动时会自动补齐
- console 中修改唤醒词时，会同步写入该文件

## ASR 说明

- 唤醒命中前，音频先只进入 wake 检测，不直接进 ASR
- 唤醒命中后，会把包含唤醒词附近的缓存帧一起送入 ASR
- 若说话结束后出现静音，会根据 endpoint 触发 final
- 若流提前结束，pipeline 也会补做一次 flush，减少尾字丢失

当前建议优先使用更高质量模型：

- `ggml-large-v3-turbo-q5_0.bin`
- `ggml-large-v3.bin`
- `ggml-medium.bin`
- `ggml-small.bin`

如果只使用 `ggml-base.bin`，中文识别准确率会明显下降。

## TTS 说明

- 当前使用 Kokoro 作为 TTS
- 声线切换支持运行时生效
- 语速可在 console 中配置
- TTS 前会做输入清洗，尽量规避 `unknown token` 和非法音素问题

## 性能说明

- 麦克风采集优先使用 `AudioWorklet`，减少主线程压力
- 若环境不支持 `AudioWorklet`，会自动回退到 `ScriptProcessor`
- 上行音频优先走 WebSocket，持续待机时比逐包 HTTP 更适合长连接场景

## 排障

### 唤醒词不起作用

1. 检查 wake 目录下是否存在 `encoder / decoder / joiner / tokens.txt`
2. 检查 `keywords.txt` 是否存在
3. 查看日志中是否出现 `[wake] embedded sherpa ready`
4. 查看是否有 `[pipeline] wake detected`

### ASR 识别缺字或截断

1. 确认是否已切到 `ggml-large-v3-turbo-q5_0.bin`
2. 观察日志中的 `[pipeline] flush utterance reason=end_of_stream ...`
3. 观察 `[asr] postprocess raw=... normalized=...`
4. 根据需要调节 `audio.endpoint_silence_frames`

### TTS 报 unknown token

1. 查看 `[tts] kokoro sanitized input ...`
2. 检查回复中是否混入异常字符或特殊音标
3. 必要时限制回复语言或继续扩展清洗规则

### 自动唤醒时 Live2D 卡顿

1. 查看是否已经使用 `audioworklet`
2. 若回退成 `scriptprocessor`，优先排查浏览器环境是否支持 worklet
3. 观察是否存在长时间未 flush 的音频上行阻塞
