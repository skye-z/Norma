const api = 'http://127.0.0.1:8090';

const moduleStatusEl = document.getElementById('module_status');
const configOverviewEl = document.getElementById('config_overview');
const eventsEl = document.getElementById('events');
const logEl = document.getElementById('log');

function log(message) {
  if (!logEl) return;
  logEl.textContent = `[${new Date().toLocaleTimeString()}] ${message}\n${logEl.textContent}`;
}

function statusClass(ok) {
  return ok ? 'state-pill' : 'state-pill danger';
}

function statusText(ok, fallback = '未知') {
  if (ok === true) return '正常';
  if (ok === false) return '异常';
  return fallback;
}

function renderModuleStatus(items) {
  moduleStatusEl.innerHTML = items.map((item) => `
    <div class="metric-tile">
      <div class="metric-label">${item.label}</div>
      <div class="metric-foot"><span class="${statusClass(item.ok)}">${statusText(item.ok, item.value || '-')}</span></div>
      <div class="metric-foot">${item.detail || '-'}</div>
    </div>
  `).join('');
}

function renderConfigOverview(items) {
  configOverviewEl.innerHTML = items.map(([label, value]) => `
    <div class="stat-line">
      <span>${label}</span>
      <strong>${value || '-'}</strong>
    </div>
  `).join('');
}

async function loadAll() {
  const [diagResp, settingsResp, providerResp, metricsResp, memoryResp] = await Promise.all([
    fetch(`${api}/runtime/diagnostics?limit=12`),
    fetch(`${api}/settings/runtime`),
    fetch(`${api}/providers/status`),
    fetch(`${api}/metrics/summary`),
    fetch(`${api}/memory?include_disabled=1&limit=5`)
  ]);

  if (!diagResp.ok) throw new Error(`读取运行时诊断失败: ${diagResp.status}`);
  if (!settingsResp.ok) throw new Error(`读取运行时配置失败: ${settingsResp.status}`);
  if (!providerResp.ok) throw new Error(`读取 Provider 状态失败: ${providerResp.status}`);
  if (!metricsResp.ok) throw new Error(`读取指标摘要失败: ${metricsResp.status}`);
  if (!memoryResp.ok) throw new Error(`读取记忆摘要失败: ${memoryResp.status}`);

  const diagnostics = await diagResp.json();
  const settings = await settingsResp.json();
  const provider = await providerResp.json();
  const metrics = await metricsResp.json();
  const memories = await memoryResp.json();

  const events = diagnostics.events || [];
  eventsEl.textContent = events.map((evt) => JSON.stringify(evt, null, 2)).join('\n\n') || '暂无事件';

  renderModuleStatus([
    {
      label: 'API 服务',
      ok: true,
      detail: '本页已通过 API 拉到诊断数据'
    },
    {
      label: '运行时状态机',
      ok: !!diagnostics.state,
      detail: `当前状态：${diagnostics.state || '-'} / 最近事件：${diagnostics.last_event || '-'}`
    },
    {
      label: 'Provider 栈',
      ok: Object.values(provider.health || {}).every((item) => item.status === 'ready'),
      detail: `Wake=${provider.stack?.wake || '-'} | ASR=${provider.stack?.asr || '-'} | TTS=${provider.stack?.tts || '-'} | LLM=${provider.stack?.llm || '-'} `
    },
    {
      label: '指标链路',
      ok: Array.isArray(metrics.counters),
      detail: `计数器 ${metrics.counters?.length || 0} / 最近样本 ${metrics.recent?.length || 0} / 告警 ${metrics.alerts?.length || 0}`
    },
    {
      label: '记忆模块',
      ok: Array.isArray(memories),
      detail: `当前拉取 ${memories.length || 0} 条摘要记录`
    }
  ]);

  renderConfigOverview([
    ['LLM Provider', settings.llm?.provider],
    ['LLM Endpoint', settings.llm?.endpoint],
    ['ASR Provider', settings.asr?.provider],
    ['TTS Provider', settings.tts?.provider],
    ['Wake Provider', settings.audio?.wake_provider],
    ['Embedding Provider', settings.rag?.embed_provider],
    ['Provider 根目录', settings.providers?.root],
    ['性能基线文件', settings.perf?.baseline_file],
    ['允许远程 Provider', settings.security?.allow_remote_providers ? '是' : '否']
  ]);

  log(`总览已刷新：事件 ${events.length} 条，Provider ${provider.manifest?.providers?.length || 0} 项，记忆 ${memories.length || 0} 条`);
}

document.getElementById('refresh')?.addEventListener('click', () => {
  loadAll().catch((error) => log(String(error)));
});

setInterval(() => {
  loadAll().catch((error) => log(String(error)));
}, 5000);

loadAll().catch((error) => log(String(error)));
