# USE

## 1. 产品说明

Norma 是一个本地优先的桌面伴侣产品。当前默认链路为：
- Wake：`sherpa-onnx`（内嵌唤醒词链路）
- ASR：`whisper.cpp`（内嵌识别）
- TTS：`Sherpa ONNX Kokoro`
- Embedding / LLM：`Ollama`

当前版本已经移除自动降级和模块管理页面，运行时以固定策略、控制台配置和日志观测为主。

## 2. 安装前准备

### 2.1 系统要求
- Windows 10/11 64 位
- 建议 16GB 以上内存
- 建议预留 8GB 以上磁盘空间给模型、日志和索引
- 首次安装建议联网，用于准备 Ollama 模型和语音资源

### 2.2 关键目录
- `config.yaml`：当前运行配置
- `config.example.yaml`：配置模板
- `resource/runtime/providers/`：运行时语音资源目录
- `resource/data/`：数据库、向量索引、记忆数据
- `resource/logs/`：运行日志
- `resource/diagnostics/`：诊断包、设备矩阵、删除证明、验收结果
- `resource/model/`：Live2D 模型与运行资源

## 3. 安装方式

### 3.1 直接使用仓库
1. 先检查编码门禁：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/check-utf8.ps1 -Root .
```

2. 如需准备内嵌语音依赖，可直接执行开发启动脚本：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/wails-dev-embedded.ps1
```

### 3.2 使用构建产物
1. 构建产物：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -Version dev -OutDir dist
```

2. 校验产物结构：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/check-release-layout.ps1 -OutDir dist
```

3. 阅读 `dist/INSTALL.md` 后启动。

## 4. 资源准备

### 4.1 控制台配置
1. 启动桌面应用，应用会自动拉起本地 API 服务。
2. 打开 [frontend/public/console/index.html](D:/Project/AI/Norma/frontend/public/console/index.html)。
3. 在控制台中确认以下配置：
   - 唤醒词与麦克风设备
   - Whisper 语言、线程数
   - Ollama endpoint 与模型
   - Kokoro 声线
   - Live2D 模型与动作组

### 4.2 推荐资源位置
- Wake 关键词与模型：`resource/runtime/providers/wake/current/`
- Whisper 模型：`resource/runtime/providers/asr/current/`
- Kokoro 模型：`resource/runtime/providers/tts/current/`
- Live2D 模型：`resource/model/`

## 5. 配置说明

### 5.1 核心配置文件
默认配置文件：`config.yaml`

重点配置项：
- `audio.wake_mode`
- `audio.wake_provider`
- `asr.provider`
- `tts.provider`
- `rag.embed_provider`
- `providers.root`

### 5.2 推荐默认值
```yaml
audio:
  wake_mode: wake_word_vad
  wake_provider: sherpa-onnx
asr:
  provider: whispercpp
tts:
  provider: sherpa-onnx
  voice: zf_xiaobei
rag:
  embed_provider: ollama
```

### 5.3 负载策略
当前版本移除了自动降级配置，默认依赖固定策略、日志观测和手动调参。

## 6. 启动方式

### 6.1 一键启动
Windows：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/start.ps1
```

Linux / macOS：
```bash
bash ./scripts/start.sh
```

### 6.2 仅做启动前检查
```powershell
powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -NoLaunch
```

### 6.3 启动主链路演示
```powershell
go run . -config config.yaml -demo
```

## 7. 首次启动推荐流程
1. 打开 [frontend/public/console/index.html](D:/Project/AI/Norma/frontend/public/console/index.html)。
2. 在控制台确认模型、麦克风、唤醒词、Ollama 和 Kokoro 配置均已填写。
3. 如需排查音频设备，先运行设备矩阵脚本：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/detect-audio-devices.ps1
```
4. 回到控制台确认状态，再启动主程序。

## 8. 如何使用

### 8.1 基础语音交互
1. 使用 `Wake Word + VAD` 模式时，先说唤醒词，再开始提问。
2. 使用 `VAD only` 模式时，可以直接说话。
3. ASR 会先输出 `partial`，结束后给出 `final`。
4. LLM 会优先给结论，再补充细节。
5. TTS 会按短句优先策略分段播报。

### 8.2 打断
- 在 `Speaking` 阶段再次说话，会触发即时打断。
- 如果音频链路不稳定，先改成 `vad_only` 排障，再逐步恢复 Wake。

### 8.3 文档问答与记忆
- 把本地文档加入索引后，可进行 RAG 问答。
- 每轮摘要会自动写入记忆系统。
- 冲突事实会被标记为待确认，可在记忆管理页查看。

### 8.4 调试页面
- 控制台：[frontend/public/console/index.html](D:/Project/AI/Norma/frontend/public/console/index.html)
- 诊断：[frontend/public/console/index.html](D:/Project/AI/Norma/frontend/public/console/index.html)

## 9. 常见排障

### 9.1 唤醒词不工作
1. 检查 `keywords.txt` 是否存在。
2. 检查 Wake 模型目录内是否同时包含 `encoder / decoder / joiner / tokens.txt`。
3. 如需先排除录音链路问题，可切换为 `vad_only`。

### 9.2 识别不出字
1. 检查 `ggml` 模型是否已放到 ASR 目录。
2. 检查 `asr.model_path` 是否正确。
3. 运行设备探测脚本采集矩阵：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/detect-audio-devices.ps1
```

### 9.3 首包过慢
1. 优先降低模型负载或缩短回答长度。
2. 确认 RAG 文档规模与索引查询是否过大。

### 9.4 外放场景回声重
1. 设置 `audio.scene=speaker`
2. 开启 AEC
3. 如果 CPU 压力过大，再对比测试关闭 NS 的效果

## 10. 诊断、清理与验收

### 10.1 采集设备矩阵
```powershell
powershell -ExecutionPolicy Bypass -File scripts/detect-audio-devices.ps1
```

### 10.2 一键清理数据与删除证明
```powershell
powershell -ExecutionPolicy Bypass -File scripts/purge-data.ps1
```

### 10.3 阶段验收
Alpha：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/alpha-smoke.ps1 -Root .
```

Beta：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/beta-smoke.ps1 -Root . -Config config.yaml
```