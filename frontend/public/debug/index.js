const api = 'http://127.0.0.1:8090';

const logEl = document.getElementById('log');
const runtimeStateEl = document.getElementById('runtime-state');
const runtimeStatsEl = document.getElementById('runtime-stats');
const healthListEl = document.getElementById('health-list');
const eventsListEl = document.getElementById('events-list');

const detailTitleEl = document.getElementById('detail-title');
const detailStatusEl = document.getElementById('detail-status');
const detailSummaryEl = document.getElementById('detail-summary');
const detailConfigEl = document.getElementById('detail-config');
const detailApplyEl = document.getElementById('detail-apply');
const detailRevertEl = document.getElementById('detail-revert');
const detailDirtyEl = document.getElementById('detail-dirty');

const statusInputEl = document.getElementById('status-input');
const statusAsrEl = document.getElementById('status-asr');
const statusRagEl = document.getElementById('status-rag');
const statusLlmEl = document.getElementById('status-llm');
const statusTtsEl = document.getElementById('status-tts');
const statusPerfEl = document.getElementById('status-perf');

const textInputEl = document.getElementById('text-input');
const textAsrEl = document.getElementById('text-asr');
const textRagEl = document.getElementById('text-rag');
const textLlmEl = document.getElementById('text-llm');
const textTtsEl = document.getElementById('text-tts');
const textPerfEl = document.getElementById('text-perf');

let currentSettings = null;
let draftSettings = null;
let currentProviderStatus = null;
let currentDiagnostics = null;
let selectedModule = 'input';
let detailDirty = false;
let saving = false;

const moduleSchemas = {
  input: {
    title: '输入模块',
    summary: '麦克风、唤醒词和前置音频策略。',
    fields: [
      { path: 'audio.enable_wake_word', label: '启用唤醒词', type: 'boolean' },
      { path: 'audio.wake_mode', label: 'Wake 模式', type: 'select', options: ['wake_word_vad', 'vad'] },
      { path: 'audio.wake_provider', label: 'Wake Provider', type: 'text' },
      { path: 'audio.wake_keyword', label: '唤醒词', type: 'text' },
      { path: 'audio.mic_device_id', label: '麦克风设备 ID', type: 'text' },
      { path: 'audio.mic_sample_rate', label: '采样率', type: 'number' }
    ]
  },
  asr: {
    title: '识别模块',
    summary: 'ASR Provider 与识别链路参数。',
    fields: [
      { path: 'asr.provider', label: 'ASR Provider', type: 'text' },
      { path: 'asr.endpoint', label: 'ASR Endpoint', type: 'text' },
      { path: 'asr.stream_endpoint', label: 'ASR Stream Endpoint', type: 'text' },
      { path: 'asr.language', label: '语言', type: 'text' },
      { path: 'asr.threads', label: '线程数', type: 'number' },
      { path: 'asr.request_timeout_ms', label: '超时(ms)', type: 'number' }
    ]
  },
  rag: {
    title: '检索模块',
    summary: 'RAG 检索与 Embedding 配置。',
    fields: [
      { path: 'rag.top_k', label: 'TopK', type: 'number' },
      { path: 'rag.embed_provider', label: 'Embedding Provider', type: 'text' },
      { path: 'rag.embed_endpoint', label: 'Embedding Endpoint', type: 'text' },
      { path: 'rag.embed_model', label: 'Embedding Model', type: 'text' },
      { path: 'rag.vector_index_path', label: '向量索引路径', type: 'text' }
    ]
  },
  llm: {
    title: '对话模块',
    summary: 'LLM Provider、模型和请求超时。',
    fields: [
      { path: 'llm.provider', label: 'LLM Provider', type: 'text' },
      { path: 'llm.endpoint', label: 'LLM Endpoint', type: 'text' },
      { path: 'llm.model', label: 'LLM Model', type: 'text' },
      { path: 'llm.request_timeout_ms', label: '超时(ms)', type: 'number' }
    ]
  },
  tts: {
    title: '输出模块',
    summary: 'TTS Provider、声线与合成策略。',
    fields: [
      { path: 'tts.provider', label: 'TTS Provider', type: 'text' },
      { path: 'tts.voice', label: 'Voice', type: 'text' },
      { path: 'tts.endpoint', label: 'TTS Endpoint', type: 'text' },
      { path: 'tts.request_timeout_ms', label: '超时(ms)', type: 'number' },
      { path: 'tts.sentence_first_enabled', label: '短句优先', type: 'boolean' }
    ]
  },
  perf: {
    title: '自动降级',
    summary: '性能阈值与自动降级策略。',
    fields: [
      { path: 'perf.auto_degrade', label: '启用自动降级', type: 'boolean' },
      { path: 'perf.llm_first_token_threshold_ms', label: '首 token 阈值(ms)', type: 'number' },
      { path: 'perf.degraded_rag_top_k', label: '降级 TopK', type: 'number' },
      { path: 'perf.degraded_llm_model', label: '降级 LLM 模型', type: 'text' },
      { path: 'perf.degraded_tts_voice', label: '降级 TTS 声线', type: 'text' }
    ]
  },
  providers: {
    title: '模块管理',
    summary: 'Provider 根目录与安全策略。',
    fields: [
      { path: 'providers.root', label: 'Provider 根目录', type: 'text' },
      { path: 'security.allow_remote_providers', label: '允许远程 Provider', type: 'boolean' },
      { path: 'security.high_risk_confirm', label: '高风险二次确认', type: 'boolean' },
      { path: 'security.purge_proof_dir', label: '删除证明目录', type: 'text' }
    ]
  }
};

function log(message) {
  if (!logEl) return;
  logEl.textContent = `[${new Date().toLocaleTimeString()}] ${message}\n${logEl.textContent}`;
}

function deepClone(value) {
  return structuredClone(value);
}

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function statusClass(status) {
  if (status === 'ready') return 'state-pill state-ready';
  if (status === 'partial') return 'state-pill state-partial';
  return 'state-pill state-missing';
}

function statusLabel(status) {
  if (status === 'ready') return '就绪';
  if (status === 'partial') return '部分就绪';
  return '缺失';
}

function indicatorClass(color) {
  return `module-status status-${color}`;
}

function formatTs(ts) {
  if (!ts) return '-';
  return new Date(ts).toLocaleTimeString();
}

function buildPayloadSummary(payload) {
  if (payload == null) return '无附加信息';
  if (typeof payload === 'string') return payload;
  if (typeof payload !== 'object') return String(payload);
  const entries = Object.entries(payload).slice(0, 4).map(([key, value]) => `${key}: ${typeof value === 'object' ? '[对象]' : String(value)}`);
  return entries.join(' / ') || '无附加信息';
}

function getValueByPath(obj, path) {
  return path.split('.').reduce((acc, key) => (acc == null ? undefined : acc[key]), obj);
}

function setValueByPath(obj, path, value) {
  const keys = path.split('.');
  let cursor = obj;
  for (let i = 0; i < keys.length - 1; i += 1) {
    const key = keys[i];
    if (!cursor[key] || typeof cursor[key] !== 'object') cursor[key] = {};
    cursor = cursor[key];
  }
  cursor[keys[keys.length - 1]] = value;
}

function setDirty(nextDirty) {
  detailDirty = !!nextDirty;
  if (detailDirtyEl) detailDirtyEl.textContent = detailDirty ? '有未保存改动' : '未修改';
  if (detailApplyEl) detailApplyEl.disabled = !detailDirty || saving;
  if (detailRevertEl) detailRevertEl.disabled = !detailDirty || saving;
}

function providerHealthStatus(providerName) {
  if (!providerName || providerName === 'mock') return null;
  const node = currentProviderStatus?.health?.[providerName];
  if (!node) return 'missing';
  return node.status || 'missing';
}

function moduleIndicatorColor(moduleKey) {
  if (!currentSettings) return 'gray';
  const stack = currentProviderStatus?.stack || {};
  switch (moduleKey) {
    case 'input': {
      if (!currentSettings.audio?.wake_provider) return 'gray';
      if (!currentSettings.audio?.enable_wake_word) return 'red';
      const st = providerHealthStatus(stack.wake || currentSettings.audio.wake_provider);
      if (st === 'ready') return 'green';
      if (st === 'partial') return 'yellow';
      if (st === 'missing') return 'gray';
      return 'yellow';
    }
    case 'asr': {
      if (!currentSettings.asr?.provider) return 'gray';
      if (currentSettings.asr.provider === 'mock') return 'red';
      const st = providerHealthStatus(stack.asr || currentSettings.asr.provider);
      if (st === 'ready') return 'green';
      if (st === 'partial') return 'yellow';
      if (st === 'missing') return 'gray';
      return 'yellow';
    }
    case 'rag': {
      if (!currentSettings.rag?.embed_provider) return 'gray';
      if (Number(currentSettings.rag?.top_k ?? 0) <= 0) return 'red';
      if (currentSettings.rag.embed_provider === 'hash') return 'green';
      const st = providerHealthStatus(stack.llm || currentSettings.llm?.provider);
      if (st === 'ready') return 'green';
      if (st === 'partial') return 'yellow';
      if (st === 'missing') return 'gray';
      return 'yellow';
    }
    case 'llm': {
      if (!currentSettings.llm?.provider) return 'gray';
      if (currentSettings.llm.provider === 'mock') return 'red';
      const st = providerHealthStatus(stack.llm || currentSettings.llm.provider);
      if (st === 'ready') return 'green';
      if (st === 'partial') return 'yellow';
      if (st === 'missing') return 'gray';
      return 'yellow';
    }
    case 'tts': {
      if (!currentSettings.tts?.provider) return 'gray';
      if (currentSettings.tts.provider === 'mock') return 'red';
      const st = providerHealthStatus(stack.tts || currentSettings.tts.provider);
      if (st === 'ready') return 'green';
      if (st === 'partial') return 'yellow';
      if (st === 'missing') return 'gray';
      return 'yellow';
    }
    case 'perf': {
      if (currentSettings.perf?.auto_degrade === undefined) return 'gray';
      return currentSettings.perf.auto_degrade ? 'green' : 'red';
    }
    case 'providers': {
      const health = Object.values(currentProviderStatus?.health || {});
      if (!health.length) return 'gray';
      if (health.every((item) => item?.status === 'ready')) return 'green';
      if (health.some((item) => item?.status === 'ready' || item?.status === 'partial')) return 'yellow';
      return 'gray';
    }
    default:
      return 'gray';
  }
}

function moduleStatus(moduleKey) {
  const color = moduleIndicatorColor(moduleKey);
  if (color === 'green') return 'ready';
  if (color === 'yellow') return 'partial';
  return 'missing';
}

function applyIndicator(el, color, label) {
  if (!el) return;
  el.className = indicatorClass(color);
  el.title = label;
}

function renderSidebarIndicators() {
  applyIndicator(statusInputEl, moduleIndicatorColor('input'), '输入模块状态');
  applyIndicator(statusAsrEl, moduleIndicatorColor('asr'), '识别模块状态');
  applyIndicator(statusRagEl, moduleIndicatorColor('rag'), '检索模块状态');
  applyIndicator(statusLlmEl, moduleIndicatorColor('llm'), '对话模块状态');
  applyIndicator(statusTtsEl, moduleIndicatorColor('tts'), '输出模块状态');
  applyIndicator(statusPerfEl, moduleIndicatorColor('perf'), '自动降级状态');
}

function renderDetailPanel() {
  if (!detailTitleEl || !detailSummaryEl || !detailConfigEl || !detailStatusEl) return;
  const schema = moduleSchemas[selectedModule] || moduleSchemas.input;
  detailTitleEl.textContent = schema.title;
  detailSummaryEl.textContent = schema.summary;
  const status = moduleStatus(selectedModule);
  detailStatusEl.className = statusClass(status);
  detailStatusEl.textContent = statusLabel(status);

  if (!draftSettings) {
    detailConfigEl.innerHTML = '<div class="stat-line"><span>状态</span><strong>配置未加载</strong></div>';
    return;
  }

  detailConfigEl.innerHTML = schema.fields.map((field) => {
    const value = getValueByPath(draftSettings, field.path);
    if (field.type === 'boolean') {
      return `
        <div class="stat-line">
          <span>${escapeHtml(field.label)}</span>
          <label><input data-path="${escapeHtml(field.path)}" type="checkbox" ${value ? 'checked' : ''} /></label>
        </div>
      `;
    }
    if (field.type === 'select') {
      return `
        <div class="stat-line">
          <span>${escapeHtml(field.label)}</span>
          <select data-path="${escapeHtml(field.path)}">
            ${(field.options || []).map((option) => `<option value="${escapeHtml(option)}" ${String(value) === String(option) ? 'selected' : ''}>${escapeHtml(option)}</option>`).join('')}
          </select>
        </div>
      `;
    }
    const inputType = field.type === 'number' ? 'number' : 'text';
    return `
      <div class="stat-line">
        <span>${escapeHtml(field.label)}</span>
        <input data-path="${escapeHtml(field.path)}" type="${inputType}" value="${escapeHtml(value ?? '')}" />
      </div>
    `;
  }).join('');
}

function renderSidebarText(settings, providerStatus) {
  const stack = providerStatus?.stack || {};
  if (textInputEl) textInputEl.textContent = `${settings?.audio?.wake_mode || 'wake_word_vad'} ${stack.wake || settings?.audio?.wake_provider || '-'}`;
  if (textAsrEl) textAsrEl.textContent = stack.asr || settings?.asr?.provider || '-';
  if (textRagEl) textRagEl.textContent = `${settings?.rag?.embed_provider || '-'} Memory`;
  if (textLlmEl) textLlmEl.textContent = `${stack.llm || settings?.llm?.provider || '-'} ${settings?.rag?.embed_provider || ''}`.trim();
  if (textTtsEl) textTtsEl.textContent = `${stack.tts || settings?.tts?.provider || '-'} ${settings?.tts?.voice || ''}`.trim();
  if (textPerfEl) textPerfEl.textContent = `${settings?.perf?.degraded_llm_model || 'default'} 阈值 ${settings?.perf?.llm_first_token_threshold_ms ?? '-'}`;
}

function renderRuntime(diagnostics, providerStatus) {
  if (!runtimeStateEl || !runtimeStatsEl) return;
  const state = diagnostics?.state || 'unknown';
  const stateText = state === 'speaking' ? '输出中' : state === 'listening' ? '输入中' : state;
  runtimeStateEl.className = statusClass(state && state !== 'error' ? 'ready' : 'missing');
  runtimeStateEl.textContent = stateText || '未知';

  const sessions = Array.isArray(diagnostics?.sessions) ? diagnostics.sessions.length : 0;
  const providerReady = Object.values(providerStatus?.health || {}).filter((item) => item?.status === 'ready').length;
  const providerTotal = Object.values(providerStatus?.health || {}).length;
  runtimeStatsEl.innerHTML = [
    ['最近事件', diagnostics?.last_event || '-'],
    ['队列深度', String(diagnostics?.queue_depth ?? 0)],
    ['会话数量', String(sessions)],
    ['Provider 就绪', `${providerReady}/${providerTotal}`]
  ].map(([k, v]) => `<div class="stat-line"><span>${escapeHtml(k)}</span><strong>${escapeHtml(v)}</strong></div>`).join('');
}

function renderHealth(providerStatus) {
  if (!healthListEl) return;
  const health = providerStatus?.health || {};
  const items = Object.entries(health);
  if (!items.length) {
    healthListEl.innerHTML = '<div class="health-item"><div class="health-title">暂无 Provider 健康信息</div></div>';
    return;
  }
  healthListEl.innerHTML = items.map(([name, item]) => `
    <div class="health-item">
      <div class="health-head">
        <div class="health-title">${escapeHtml(name)}</div>
        <span class="${statusClass(item?.status)}">${statusLabel(item?.status)}</span>
      </div>
      <div class="health-detail">找到: ${escapeHtml((item?.found || []).join('、') || '无')}</div>
      <div class="health-detail">缺失: ${escapeHtml((item?.missing || []).join('、') || '无')}</div>
    </div>
  `).join('');
}

function renderEvents(events) {
  if (!eventsListEl) return;
  const filtered = (events || []).filter((evt) => evt?.name !== 'live2d.eye').slice(-10).reverse();
  if (!filtered.length) {
    eventsListEl.innerHTML = '<div class="event-item"><div class="event-title">暂无事件</div></div>';
    return;
  }
  eventsListEl.innerHTML = filtered.map((evt) => `
    <div class="event-item">
      <div class="event-head">
        <div class="event-title">${escapeHtml(evt?.name || '未知事件')}</div>
        <div class="event-time">${escapeHtml(formatTs(evt?.ts_unix_ms))}</div>
      </div>
      <div class="event-detail">${escapeHtml(buildPayloadSummary(evt?.payload))}</div>
    </div>
  `).join('');
}

async function saveSettings(nextSettings, successMessage) {
  if (saving) return;
  saving = true;
  if (detailApplyEl) detailApplyEl.disabled = true;
  if (detailRevertEl) detailRevertEl.disabled = true;
  try {
    const resp = await fetch(`${api}/settings/runtime`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(nextSettings)
    });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok || data?.ok !== true) {
      throw new Error(data?.error?.message || `保存配置失败: ${resp.status}`);
    }
    currentSettings = deepClone(nextSettings);
    draftSettings = deepClone(nextSettings);
    setDirty(false);
    log(successMessage);
    await loadAll(false);
  } catch (error) {
    log(String(error));
  } finally {
    saving = false;
    setDirty(detailDirty);
  }
}

function selectModule(moduleKey) {
  selectedModule = moduleKey;
  document.querySelectorAll('.console-row[data-module]').forEach((row) => {
    row.classList.toggle('active', row.dataset.module === selectedModule);
  });
  renderDetailPanel();
}

function bindSidebarModuleNav() {
  document.querySelectorAll('.console-row[data-module]').forEach((row) => {
    row.addEventListener('click', (event) => {
      if (event.target.closest('button,input,select')) return;
      const moduleKey = row.dataset.module;
      if (moduleKey) selectModule(moduleKey);
    });
  });
}

function bindDetailEditor() {
  detailConfigEl?.addEventListener('change', (event) => {
    if (!draftSettings) return;
    const target = event.target;
    if (!target || !target.dataset?.path) return;
    const path = target.dataset.path;
    const schema = moduleSchemas[selectedModule];
    const field = schema?.fields?.find((item) => item.path === path);
    if (!field) return;

    let nextValue;
    if (field.type === 'boolean') {
      nextValue = !!target.checked;
    } else if (field.type === 'number') {
      const num = Number(target.value);
      if (!Number.isFinite(num)) return;
      nextValue = num;
    } else {
      nextValue = String(target.value || '').trim();
    }

    setValueByPath(draftSettings, path, nextValue);
    setDirty(true);
    renderDetailPanel();
  });

  detailApplyEl?.addEventListener('click', async () => {
    if (!detailDirty || !draftSettings) return;
    await saveSettings(deepClone(draftSettings), `已应用 ${moduleSchemas[selectedModule]?.title || '模块'} 配置改动`);
  });

  detailRevertEl?.addEventListener('click', () => {
    if (!currentSettings) return;
    draftSettings = deepClone(currentSettings);
    setDirty(false);
    renderDetailPanel();
    log(`已撤销 ${moduleSchemas[selectedModule]?.title || '模块'} 未保存改动`);
  });
}

async function loadAll(appendLog = true) {
  const [diagResp, settingsResp, providerResp] = await Promise.all([
    fetch(`${api}/runtime/diagnostics?limit=20`),
    fetch(`${api}/settings/runtime`),
    fetch(`${api}/providers/status`)
  ]);
  if (!diagResp.ok) throw new Error(`读取诊断失败: ${diagResp.status}`);
  if (!settingsResp.ok) throw new Error(`读取配置失败: ${settingsResp.status}`);
  if (!providerResp.ok) throw new Error(`读取 Provider 状态失败: ${providerResp.status}`);

  currentDiagnostics = await diagResp.json();
  const settings = await settingsResp.json();
  currentProviderStatus = await providerResp.json();

  if (!detailDirty || !currentSettings) {
    currentSettings = settings;
    draftSettings = deepClone(settings);
    setDirty(false);
  }

  renderSidebarText(currentSettings, currentProviderStatus);
  renderSidebarIndicators();
  renderRuntime(currentDiagnostics, currentProviderStatus);
  renderHealth(currentProviderStatus);
  renderEvents(currentDiagnostics?.events || []);
  renderDetailPanel();

  if (appendLog) {
    log(`已刷新: state=${currentDiagnostics?.state || '-'} queue=${currentDiagnostics?.queue_depth ?? 0}`);
  }
}

document.getElementById('refresh')?.addEventListener('click', () => {
  loadAll().catch((error) => log(String(error)));
});

bindSidebarModuleNav();
bindDetailEditor();
selectModule(selectedModule);
loadAll().catch((error) => log(String(error)));
setInterval(() => {
  loadAll().catch((error) => log(String(error)));
}, 5000);
