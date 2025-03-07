# Norma - 诺玛

> 一个基于 Golang + Live2D 的桌面人格化伴侣开源项目。

Norma 当前目标不是 MVP，而是一个可长期使用、可发布、可运维、可持续迭代的完整桌面伴侣产品。

## 产品定位

Norma 以 Go 为核心，结合 Live2D、语音交互、本地大模型、记忆系统和本地知识检索，打造本地优先的桌面虚拟伙伴。

核心方向：
1. 高拟人化交互：动作、表情、口型、情绪联动。
2. 文本 + 语音双通道：支持唤醒、VAD、ASR、TTS、可打断交互。
3. 记忆与知识增强：STM/LTM + 本地文件索引 + Hybrid RAG。
4. 极简部署：单机运行，不依赖 Redis/ES 等外部中间件。
5. 发布级运维：自检、诊断包、日志分层、依赖自愈、质量门禁。

## 当前架构

1. 运行时内核：状态机、事件总线、取消机制、运行时观测。
2. 语音链路：Wake Word/VAD、端点检测、流式 ASR、分句 TTS、打断闭环。
3. 记忆系统：STM token 预算裁剪、LTM 字段化存储、治理任务、管理接口。
4. RAG：本地文件抽取、结构化分块、混合检索、增量索引。
5. 运维：启动链路、watchdog、诊断包、发布前检查。

## 运行方式

### 启动应用

```powershell
powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -Root .
```

### 仅执行预检查和依赖检查

```powershell
powershell -ExecutionPolicy Bypass -File scripts/start.ps1 -Root . -NoLaunch
```

### API 与开发调试面板

桌面程序启动时会自动拉起本地 API 服务，无需再单独启动 `cmd/apiserver`。

开发调试面板入口：
- 静态调试页：[web/debug/index.html](D:/Project/AI/Norma/web/debug/index.html)
- 运行时地址：`http://127.0.0.1:8090/static/debug/index.html`

## 关键文档

1. [开发计划](D:/Project/AI/Norma/docs/开发计划.md)
2. [方案](D:/Project/AI/Norma/docs/方案.md)
3. [部署与运维](D:/Project/AI/Norma/docs/部署与运维.md)
4. [发布与首启指南](D:/Project/AI/Norma/docs/发布与首启指南.md)
5. [记忆系统设计](D:/Project/AI/Norma/docs/记忆系统设计.md)
6. [语音交互模块设计](D:/Project/AI/Norma/docs/语音交互模块设计.md)

## 当前状态

已完成的关键能力：
1. Hybrid Retriever 主链路接入。
2. 文档提取扩展到 `txt/md/pdf/docx`。
3. 结构化分块、检索去重/合并/轻量重排、增量索引治理。
4. 运行时事件协议、取消机制、延迟观测和启动自检。
5. 开发模式调试面板、记忆 API 与基础管理能力。

仍在推进的重点：
1. 全仓乱码治理与 UTF-8 门禁。
2. 语音链路产品化闭环。
3. 记忆系统摘要/冲突/治理深度完善。
4. 跨平台发布工程与质量封板。
