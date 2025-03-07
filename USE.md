# USE

## 1. 产品说明

Norma 是一个本地优先的桌面伴侣产品。当前默认链路为：
- Wake：`sherpa-onnx`
- ASR：`whisper.cpp`
- TTS：`Piper`
- Embedding：`Ollama`

这些 Provider 会尽量纳入 Norma 统一管理，避免用户自行下载安装、手工改路径和分散排障。

## 2. 安装前准备

### 2.1 系统要求
- Windows 10/11 64 位
- 建议 16GB 以上内存
- 建议预留 8GB 以上磁盘空间给模型、日志和索引
- 首次安装建议联网，用于下载 Provider 和模型

### 2.2 关键目录
- `config.yaml`：当前运行配置
- `config.example.yaml`：配置模板
- `resource/runtime/providers/`：受管 Provider 与模型目录
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

2. 检查 Provider 根目录：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/provider-manage.ps1 -Action status
```

3. 如果 Provider 目录尚未初始化，可在设置页点击“初始化 Provider 目录”，或调用：
```powershell
curl -X POST http://127.0.0.1:8090/providers/init
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

## 4. 安装 Provider 与模型

### 4.1 通过设置页安装
1. 启动桌面应用，应用会自动拉起本地 API 服务。
2. 打开 [web/debug/settings.html](D:/Project/AI/Norma/web/debug/settings.html)。
3. 在“Provider 面板”中依次安装：
   - `wake-sherpa-onnx-windows-x64-shared`
   - `wake-sherpa-kws-zh-en-model`
   - `asr-whispercpp-windows-x64`
   - `asr-whisper-model-base`
   - `tts-piper-windows-x64`
   - `tts-piper-voice-zh-cn-huayan-medium`
   - `tts-piper-voice-zh-cn-huayan-x-low`
   - `llm-ollama-windows-x64`
4. 点击“应用受管 Provider”，把路径和默认值自动回填到 `config.yaml`。

### 4.2 通过脚本安装
安装示例：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/provider-install.ps1 -CatalogPath configs/provider-catalog.json -ProviderId asr-whispercpp-windows-x64 -ProvidersRoot resource/runtime/providers
```

卸载示例：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/provider-install.ps1 -CatalogPath configs/provider-catalog.json -ProviderId asr-whispercpp-windows-x64 -ProvidersRoot resource/runtime/providers -Uninstall
```

### 4.3 来源与校验说明
- `sherpa-onnx` 已接入官方 `checksum.txt` 校验，安装时会自动解析并验证 SHA256。
- `whisper.cpp / Piper / Ollama` 当前只能确认“官方来源”，尚不能确认上游发布了可直接接入的 checksum 清单。
- 对于没有官方 checksum 的 Provider，Norma 会在安装时记录：
  - 下载来源 URL
  - 安装时间
  - 主文件本地 SHA256
  - 额外文件本地 SHA256
- 设置页会明确区分“已接入官方 SHA256 校验”和“仅官方来源 + 本地 SHA256 留档”，不要把两者视为同一等级。
- 就绪报告导出后，同样会包含这些来源审计字段，便于排障和归档。

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
  provider: piper
  voice: zh_CN-huayan-medium
rag:
  embed_provider: ollama
```

### 5.3 高负载自动降级配置
用于降低 TTFA 和资源占用：

```yaml
perf:
  auto_degrade: true
  llm_first_token_threshold_ms: 1200
  degraded_rag_top_k: 4
  degraded_ef_search: 24
  degraded_retriever_extra_factor: 1
  degraded_retriever_fuse_factor: 2
  degraded_audio_chunk_size: 4096
  degraded_llm_model: qwen2.5:3b
  degraded_tts_voice: zh_CN-huayan-x_low
```

说明：
- `degraded_llm_model`：高负载时切换到更轻的 Ollama 模型
- `degraded_tts_voice`：高负载时切换到更轻的 Piper 音色
- `degraded_ef_search / degraded_rag_top_k`：降低检索成本，优先保住首答速度

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
go run ./cmd/norma -config config.yaml -demo
```

## 7. 首次启动推荐流程
1. 打开 [web/debug/onboarding.html](D:/Project/AI/Norma/web/debug/onboarding.html)。
2. 检查以下项目是否就绪：
   - Wake / ASR / TTS Provider
   - Live2D 模型
   - 配置文件
   - 数据目录
   - 设备探测矩阵脚本
3. 如果有缺失，先到 [web/debug/settings.html](D:/Project/AI/Norma/web/debug/settings.html) 安装并应用受管 Provider。
4. 回到首启向导确认状态，再启动主程序。

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
- 运行时诊断：[web/debug/index.html](D:/Project/AI/Norma/web/debug/index.html)
- 记忆管理：[web/debug/memory.html](D:/Project/AI/Norma/web/debug/memory.html)
- 指标看板：[web/debug/metrics.html](D:/Project/AI/Norma/web/debug/metrics.html)
- 运行时设置：[web/debug/settings.html](D:/Project/AI/Norma/web/debug/settings.html)
- 首启向导：[web/debug/onboarding.html](D:/Project/AI/Norma/web/debug/onboarding.html)

## 9. 常见排障

### 9.1 唤醒词不工作
1. 先看设置页中 Wake 是否为 `ready` 或 `partial`。
2. 检查 `keywords.txt` 是否存在。
3. 检查 Wake 模型目录内是否同时包含 `encoder / decoder / joiner / tokens.txt`。
4. 如需先排除录音链路问题，可切换为 `vad_only`。

### 9.2 识别不出字
1. 检查 `whisper-cli.exe` 和 `ggml-base.bin` 是否都已安装。
2. 检查 `asr.binary_path / asr.model_path` 是否已自动回填。
3. 运行设备探测脚本采集矩阵：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/detect-audio-devices.ps1
```

### 9.3 首包过慢
1. 打开自动降级。
2. 配置 `degraded_llm_model`。
3. 将 `degraded_tts_voice` 改为 `zh_CN-huayan-x_low`。
4. 降低 `degraded_rag_top_k` 和 `degraded_ef_search`。

### 9.4 外放场景回声重
1. 设置 `audio.scene=speaker`
2. 开启 AEC
3. 如果 CPU 压力过大，再对比测试关闭 NS 的效果

## 10. 诊断、清理与验收

### 10.1 导出 Provider 就绪报告
可在设置页点击“导出就绪报告”，或调用：
```powershell
curl -X POST http://127.0.0.1:8090/providers/export-readiness
```

### 10.2 采集设备矩阵
```powershell
powershell -ExecutionPolicy Bypass -File scripts/detect-audio-devices.ps1
```

### 10.3 一键清理数据与删除证明
```powershell
powershell -ExecutionPolicy Bypass -File scripts/purge-data.ps1
```

### 10.4 阶段验收
Alpha：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/alpha-smoke.ps1 -Root .
```

Beta：
```powershell
powershell -ExecutionPolicy Bypass -File scripts/beta-smoke.ps1 -Root . -Config config.yaml
```
