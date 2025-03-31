let api = window.location.origin && window.location.origin.startsWith('http')
  ? window.location.origin
  : 'http://127.0.0.1:8090';
const fallbackApi = 'http://127.0.0.1:8090';

async function detectAPIBase() {
  const candidates = [api, fallbackApi].filter((value, index, arr) => value && arr.indexOf(value) === index);
  for (const candidate of candidates) {
    try {
      const resp = await fetch(`${candidate}/healthz`, { cache: 'no-store' });
      if (!resp.ok) continue;
      const ctype = String(resp.headers.get('content-type') || '').toLowerCase();
      if (!ctype.includes('application/json')) continue;
      const data = await resp.json().catch(() => null);
      if (data && typeof data === 'object') return candidate;
    } catch (_) {}
  }
  return fallbackApi;
}

const detailTitleEl = document.getElementById('detail-title');
const detailStatusEl = document.getElementById('detail-status');
const detailSummaryEl = document.getElementById('detail-summary');
const detailConfigEl = document.getElementById('detail-config');
const detailApplyEl = document.getElementById('detail-apply');
const detailRevertEl = document.getElementById('detail-revert');
const detailDirtyEl = document.getElementById('detail-dirty');

const statusModelEl = document.getElementById('status-model');
const statusInputEl = document.getElementById('status-input');
const statusAsrEl = document.getElementById('status-asr');
const statusRagEl = document.getElementById('status-rag');
const statusLlmEl = document.getElementById('status-llm');
const statusTtsEl = document.getElementById('status-tts');
const statusPerfEl = document.getElementById('status-perf');

const statusTextModelEl = document.getElementById('status-text-model');
const statusTextInputEl = document.getElementById('status-text-input');
const statusTextAsrEl = document.getElementById('status-text-asr');
const statusTextRagEl = document.getElementById('status-text-rag');
const statusTextLlmEl = document.getElementById('status-text-llm');
const statusTextTtsEl = document.getElementById('status-text-tts');
const statusTextPerfEl = document.getElementById('status-text-perf');

const textModelEl = document.getElementById('text-model');
const textInputEl = document.getElementById('text-input');
const textAsrEl = document.getElementById('text-asr');
const textRagEl = document.getElementById('text-rag');
const textLlmEl = document.getElementById('text-llm');
const textTtsEl = document.getElementById('text-tts');
const textPerfEl = document.getElementById('text-perf');

const modelStorageKey = 'norma.debug.model_url';
let modelCatalog = [];

const componentCatalog = [
  { key: 'ollama', name: 'Ollama', desc: '用于大模型和向量嵌入' },
  { key: 'piper', name: 'Piper', desc: '用于语音合成' },
  { key: 'whispercpp', name: 'Whisper.cpp', desc: '用于语音识别' },
  { key: 'sherpa-onnx', name: 'Sherpa ONNX', desc: '用于唤醒词' }
];

const moduleSchemas = {
  model: {
    title: '模型模块',
    summary: '切换 Live2D 模型，读取该模型动作组并手动触发动作和姿态。',
    fields: [
      { path: 'debug.model_url', label: '模型文件', type: 'model_select' },
      { path: 'debug.motion_group', label: '动作组', type: 'motion_select' },
      { path: 'debug.motion_index', label: '动作序号', type: 'number' },
      { path: 'debug.eye_x', label: '视线 X', type: 'number' },
      { path: 'debug.eye_y', label: '视线 Y', type: 'number' },
      { path: 'debug.mouth_level', label: '口型强度', type: 'number' }
    ]
  },
  input: {
    title: '输入模块',
    summary: '麦克风、唤醒词和前置音频策略。',
    fields: [
      { path: 'audio.enable_wake_word', label: '启用唤醒词', type: 'boolean' },
      { path: 'audio.wake_mode', label: '唤醒模式', type: 'select', options: [{ value: 'wake_word_vad', label: '唤醒词+语音端点检测' }, { value: 'vad', label: '仅语音端点检测' }] },
      { path: 'audio.wake_provider', label: '唤醒组件', type: 'select', options: [{ value: 'sherpa-onnx', label: 'Sherpa ONNX' }] },
      { path: 'audio.wake_keyword', label: '唤醒词', type: 'text' },
      { path: 'audio.mic_device_id', label: '麦克风设备', type: 'text' },
      { path: 'audio.mic_sample_rate', label: '采样率', type: 'number' }
    ]
  },
  asr: {
    title: '识别模块',
    summary: '语音识别配置。',
    fields: [
      { path: 'asr.provider', label: '识别组件', type: 'select', options: [{ value: 'whispercpp', label: 'Whisper.cpp' }] },
      { path: 'asr.endpoint', label: '识别接口地址', type: 'text' },
      { path: 'asr.stream_endpoint', label: '流式识别接口', type: 'text' },
      { path: 'asr.language', label: '识别语言', type: 'text' },
      { path: 'asr.threads', label: '线程数', type: 'number' },
      { path: 'asr.request_timeout_ms', label: '超时(毫秒)', type: 'number' }
    ]
  },
  rag: {
    title: '检索模块',
    summary: '知识检索和向量嵌入配置。',
    fields: [
      { path: 'rag.top_k', label: '召回条数', type: 'number' },
      { path: 'rag.embed_provider', label: '嵌入组件', type: 'select', options: [{ value: 'ollama', label: 'Ollama' }] },
      { path: 'rag.embed_endpoint', label: '嵌入接口地址', type: 'text' },
      { path: 'rag.embed_model', label: '嵌入模型', type: 'text' },
      { path: 'rag.vector_index_path', label: '向量索引路径', type: 'text' }
    ]
  },
  llm: {
    title: '对话模块',
    summary: '大模型推理配置。',
    fields: [
      { path: 'llm.provider', label: '大模型组件', type: 'select', options: [{ value: 'ollama', label: 'Ollama' }] },
      { path: 'llm.endpoint', label: '大模型接口地址', type: 'text' },
      { path: 'llm.model', label: '模型名称', type: 'text' },
      { path: 'llm.request_timeout_ms', label: '超时(毫秒)', type: 'number' }
    ]
  },
  tts: {
    title: '输出模块',
    summary: '语音合成配置。',
    fields: [
      { path: 'tts.provider', label: '合成组件', type: 'select', options: [{ value: 'piper', label: 'Piper' }] },
      { path: 'tts.voice', label: '声线', type: 'text' },
      { path: 'tts.endpoint', label: '合成接口地址', type: 'text' },
      { path: 'tts.request_timeout_ms', label: '超时(毫秒)', type: 'number' },
      { path: 'tts.sentence_first_enabled', label: '启用短句优先', type: 'boolean' }
    ]
  },
  perf: {
    title: '自动降级',
    summary: '超时阈值和降级策略。',
    fields: [
      { path: 'perf.auto_degrade', label: '启用自动降级', type: 'boolean' },
      { path: 'perf.llm_first_token_threshold_ms', label: '首 Token 阈值(毫秒)', type: 'number' },
      { path: 'perf.degraded_rag_top_k', label: '降级后召回条数', type: 'number' },
      { path: 'perf.degraded_llm_model', label: '降级后模型', type: 'text' },
      { path: 'perf.degraded_tts_voice', label: '降级后声线', type: 'text' }
    ]
  },
  providers: {
    title: '模块管理',
    summary: '固定组件的发现、安装、更新、卸载和配置。',
    fields: []
  }
};

let currentSettings = null;
let draftSettings = null;
let currentProviderStatus = null;
let discoverItems = [];
let selectedModule = 'model';
let detailDirty = false;
let saving = false;
let modelMotionGroups = ['Idle'];

function log(message) {
  console.log(`[debug-console] ${message}`);
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
  if (status === 'ready') return '已就绪';
  if (status === 'partial') return '不可用';
  return '未安装';
}

function indicatorClass(color) {
  return `module-status status-${color}`;
}

function indicatorText(color) {
  if (color === 'green') return '已就绪';
  if (color === 'yellow') return '不可用';
  if (color === 'red') return '未启用';
  return '未安装';
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

function ensureDebugDraft() {
  if (!draftSettings) return;
  if (!draftSettings.audio) draftSettings.audio = {};
  if (!draftSettings.asr) draftSettings.asr = {};
  if (!draftSettings.tts) draftSettings.tts = {};
  if (!draftSettings.llm) draftSettings.llm = {};
  if (!draftSettings.rag) draftSettings.rag = {};
  if (!draftSettings.perf) draftSettings.perf = {};

  if (!draftSettings.audio.wake_provider) draftSettings.audio.wake_provider = 'sherpa-onnx';
  if (!draftSettings.asr.provider) draftSettings.asr.provider = 'whispercpp';
  if (!draftSettings.tts.provider) draftSettings.tts.provider = 'piper';
  if (!draftSettings.llm.provider) draftSettings.llm.provider = 'ollama';
  if (!draftSettings.rag.embed_provider) draftSettings.rag.embed_provider = 'ollama';
  if (!draftSettings.audio.wake_mode) draftSettings.audio.wake_mode = 'wake_word_vad';
  if (!draftSettings.audio.wake_keyword) draftSettings.audio.wake_keyword = '诺玛';
  if (!draftSettings.asr.language) draftSettings.asr.language = 'zh';
  if (!draftSettings.asr.threads) draftSettings.asr.threads = 4;
  if (!draftSettings.rag.top_k) draftSettings.rag.top_k = 6;

  if (!draftSettings.debug || typeof draftSettings.debug !== 'object') {
    draftSettings.debug = {};
  }
  if (!draftSettings.debug.model_url) {
    const stored = localStorage.getItem(modelStorageKey) || '';
    draftSettings.debug.model_url = stored || (modelCatalog[0]?.url || '');
  }
  if (!draftSettings.debug.motion_group) draftSettings.debug.motion_group = 'Idle';
  if (typeof draftSettings.debug.motion_index !== 'number') draftSettings.debug.motion_index = 0;
  if (typeof draftSettings.debug.eye_x !== 'number') draftSettings.debug.eye_x = 0;
  if (typeof draftSettings.debug.eye_y !== 'number') draftSettings.debug.eye_y = 0;
  if (typeof draftSettings.debug.mouth_level !== 'number') draftSettings.debug.mouth_level = 0.6;
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
    case 'model': {
      if (!draftSettings?.debug?.model_url) return 'gray';
      if (!modelMotionGroups.length) return 'yellow';
      return 'green';
    }
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

function applyIndicator(el, textEl, color, label) {
  if (el) {
    el.className = indicatorClass(color);
    el.title = label;
  }
  if (textEl) textEl.textContent = indicatorText(color);
}

function renderSidebarIndicators() {
  applyIndicator(statusModelEl, statusTextModelEl, moduleIndicatorColor('model'), '模型状态');
  applyIndicator(statusInputEl, statusTextInputEl, moduleIndicatorColor('input'), '输入状态');
  applyIndicator(statusAsrEl, statusTextAsrEl, moduleIndicatorColor('asr'), '识别状态');
  applyIndicator(statusRagEl, statusTextRagEl, moduleIndicatorColor('rag'), '检索状态');
  applyIndicator(statusLlmEl, statusTextLlmEl, moduleIndicatorColor('llm'), '对话状态');
  applyIndicator(statusTtsEl, statusTextTtsEl, moduleIndicatorColor('tts'), '输出状态');
  applyIndicator(statusPerfEl, statusTextPerfEl, moduleIndicatorColor('perf'), '自动降级状态');
}

function renderSidebarText(settings, providerStatus) {
  const stack = providerStatus?.stack || {};
  const wakeModeMap = {
    wake_word_vad: '唤醒词+端点检测',
    vad: '仅端点检测'
  };
  const activeURL = getValueByPath(draftSettings, 'debug.model_url') || (modelCatalog[0]?.url || '');
  const hit = modelCatalog.find((item) => item.url === activeURL);
  if (textModelEl) textModelEl.textContent = hit?.label || '自定义模型';
  if (textInputEl) textInputEl.textContent = `${wakeModeMap[settings?.audio?.wake_mode] || '输入策略'} / ${stack.wake || settings?.audio?.wake_provider || '-'}`;
  if (textAsrEl) textAsrEl.textContent = stack.asr || settings?.asr?.provider || '-';
  if (textRagEl) textRagEl.textContent = `${settings?.rag?.embed_provider || '-'} / 检索条数 ${settings?.rag?.top_k ?? '-'}`;
  if (textLlmEl) textLlmEl.textContent = `${stack.llm || settings?.llm?.provider || '-'} / ${settings?.llm?.model || '默认模型'}`;
  if (textTtsEl) textTtsEl.textContent = `${stack.tts || settings?.tts?.provider || '-'} / ${settings?.tts?.voice || '默认声线'}`;
  if (textPerfEl) textPerfEl.textContent = `${settings?.perf?.auto_degrade ? '已启用' : '未启用'} / 阈值 ${settings?.perf?.llm_first_token_threshold_ms ?? '-'}`;
}

async function loadMotionGroups(modelURL) {
  if (!String(modelURL || '').trim()) {
    modelMotionGroups = ['Idle'];
    return;
  }
  try {
    const resp = await fetch(modelURL, { cache: 'no-store' });
    if (!resp.ok) throw new Error(`读取模型文件失败: ${resp.status}`);
    const json = await resp.json();
    const motions = json?.FileReferences?.Motions || {};
    const groups = Object.keys(motions).filter(Boolean);
    modelMotionGroups = groups.length ? groups : ['Idle'];
    const current = getValueByPath(draftSettings, 'debug.motion_group');
    if (!modelMotionGroups.includes(current)) {
      setValueByPath(draftSettings, 'debug.motion_group', modelMotionGroups[0]);
    }
  } catch (_) {
    modelMotionGroups = ['Idle'];
  }
}

function optionListHTML(options, currentValue) {
  return options.map((opt) => {
    const value = typeof opt === 'object' ? (opt.value ?? opt.url ?? '') : opt;
    const label = typeof opt === 'object' ? (opt.label ?? opt.name ?? String(value)) : opt;
    return `<option value="${escapeHtml(value)}" ${String(currentValue) === String(value) ? 'selected' : ''}>${escapeHtml(label)}</option>`;
  }).join('');
}

function inferModelNameFromURL(url) {
  const raw = String(url || '').trim();
  if (!raw) return '';
  const marker = '/static/model/';
  const idx = raw.indexOf(marker);
  if (idx < 0) return '';
  const rest = raw.slice(idx + marker.length);
  const parts = rest.split('/').filter(Boolean);
  return parts.length ? parts[0] : '';
}

function renderProviderManageList() {
  if (selectedModule !== 'providers') return '';
  const merged = componentCatalog.map((base) => {
    const found = discoverItems.find((item) => item.key === base.key);
    return {
      ...base,
      installed: !!found?.installed,
      paths: found?.paths || [],
      hint: found?.hint || base.desc
    };
  });

  return `
    <div class="provider-manage-grid">
      ${merged.map((item) => `
        <div class="provider-item">
          <div class="provider-head">
            <strong>${escapeHtml(item.name)}</strong>
            <span>${item.installed ? '已安装' : '未安装'}</span>
          </div>
          <div class="event-detail">${escapeHtml(item.hint)}</div>
          <div class="event-detail">${escapeHtml(item.paths[0] || '未检测到可执行文件')}</div>
          <div class="provider-actions">
            <button data-provider-key="${escapeHtml(item.key)}" data-provider-action="install" type="button" ${item.installed ? 'disabled' : ''}>安装</button>
            <button data-provider-key="${escapeHtml(item.key)}" data-provider-action="update" type="button" ${item.installed ? '' : 'disabled'}>更新</button>
            <button data-provider-key="${escapeHtml(item.key)}" data-provider-action="uninstall" type="button" ${item.installed ? '' : 'disabled'}>卸载</button>
            <button data-provider-key="${escapeHtml(item.key)}" data-provider-action="configure" type="button" ${item.installed ? '' : 'disabled'}>应用配置</button>
          </div>
        </div>
      `).join('')}
      <button data-provider-action="discover" type="button">重新发现已安装组件</button>
    </div>
  `;
}

function renderDetailPanel() {
  if (!detailTitleEl || !detailSummaryEl || !detailConfigEl || !detailStatusEl || !draftSettings) return;
  const schema = moduleSchemas[selectedModule] || moduleSchemas.input;
  detailTitleEl.textContent = schema.title;
  detailSummaryEl.textContent = schema.summary;

  const status = moduleStatus(selectedModule);
  detailStatusEl.className = statusClass(status);
  detailStatusEl.textContent = statusLabel(status);

  const hideDraftButtons = selectedModule === 'model' || selectedModule === 'providers';
  if (detailApplyEl) detailApplyEl.style.display = hideDraftButtons ? 'none' : '';
  if (detailRevertEl) detailRevertEl.style.display = hideDraftButtons ? 'none' : '';
  if (detailDirtyEl) detailDirtyEl.style.display = hideDraftButtons ? 'none' : '';

  const formHTML = schema.fields.map((field) => {
    const value = getValueByPath(draftSettings, field.path);
    if (field.type === 'boolean') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><label><input data-path="${escapeHtml(field.path)}" type="checkbox" ${value ? 'checked' : ''} /></label></div>`;
    }
    if (field.type === 'select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(field.options || [], value)}</select></div>`;
    }
    if (field.type === 'model_select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(modelCatalog, value)}</select></div>`;
    }
    if (field.type === 'motion_select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(modelMotionGroups, value)}</select></div>`;
    }
    const inputType = field.type === 'number' ? 'number' : 'text';
    return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><input data-path="${escapeHtml(field.path)}" type="${inputType}" value="${escapeHtml(value ?? '')}" /></div>`;
  }).join('');

  const modelActions = selectedModule === 'model'
    ? `<div class="model-action-grid"><button data-model-action="reload" type="button">加载所选模型</button><button data-model-action="pose" type="button">发送姿态</button><button data-model-action="mouth" type="button">发送口型</button></div><div class="event-detail">动作组下拉切换后会自动发送动作</div>`
    : '';

  detailConfigEl.innerHTML = `${formHTML}${modelActions}${renderProviderManageList()}`;
}

function setDirty(nextDirty) {
  detailDirty = !!nextDirty;
  if (detailDirtyEl) detailDirtyEl.textContent = detailDirty ? '有未保存改动' : '未修改';
  if (detailApplyEl) detailApplyEl.disabled = !detailDirty || saving;
  if (detailRevertEl) detailRevertEl.disabled = !detailDirty || saving;
}

function sanitizeSettingsForSave(cfg) {
  const clean = deepClone(cfg);
  if (clean.debug) delete clean.debug;
  return clean;
}

async function saveSettings(nextDraft, successMessage) {
  if (saving) return;
  saving = true;
  if (detailApplyEl) detailApplyEl.disabled = true;
  if (detailRevertEl) detailRevertEl.disabled = true;
  try {
    const payload = sanitizeSettingsForSave(nextDraft);
    const resp = await fetch(`${api}/settings/runtime`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok || data?.ok !== true) {
      throw new Error(data?.error?.message || `保存配置失败: ${resp.status}`);
    }
    currentSettings = deepClone(payload);
    currentSettings.debug = deepClone(nextDraft.debug || {});
    draftSettings = deepClone(currentSettings);
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

async function publishRuntimeEvent(name, payload) {
  const resp = await fetch(`${api}/runtime/publish`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, session_id: 'debug-console', trace_id: 'debug-console', payload })
  });
  if (!resp.ok) throw new Error(`发送事件失败: ${name}`);
}

async function runModelAction(kind) {
  const modelURL = String(getValueByPath(draftSettings, 'debug.model_url') || (modelCatalog[0]?.url || '')).trim();
  const motion = String(getValueByPath(draftSettings, 'debug.motion_group') || 'Idle').trim();
  const motionIndex = Number(getValueByPath(draftSettings, 'debug.motion_index') || 0);
  const eyeX = Number(getValueByPath(draftSettings, 'debug.eye_x') || 0);
  const eyeY = Number(getValueByPath(draftSettings, 'debug.eye_y') || 0);
  const mouth = Number(getValueByPath(draftSettings, 'debug.mouth_level') || 0.6);

  if (kind === 'reload') {
    if (!modelURL) throw new Error('当前没有可用模型文件，请先在“模型文件”里选择有效项');
    localStorage.setItem(modelStorageKey, modelURL);
    await publishRuntimeEvent('live2d.reload', { source: 'debug-console', model_url: modelURL });
    log(`已触发模型重载: ${modelURL}`);
    return;
  }
  if (kind === 'motion') {
    await publishRuntimeEvent('live2d.motion', { group: motion, index: Number.isFinite(motionIndex) ? motionIndex : 0 });
    log(`已发送动作: ${motion}`);
    return;
  }
  if (kind === 'pose') {
    await publishRuntimeEvent('live2d.eye', { x: Number.isFinite(eyeX) ? eyeX : 0, y: Number.isFinite(eyeY) ? eyeY : 0 });
    log('已发送视线姿态');
    return;
  }
  if (kind === 'mouth') {
    await publishRuntimeEvent('live2d.mouth', { level: Number.isFinite(mouth) ? mouth : 0.6 });
    log('已发送口型事件');
  }
}

async function runProviderAction(action, key) {
  if (action === 'discover') {
    await loadDiscoverOnly();
    renderDetailPanel();
    return;
  }
  if (action === 'configure') {
    const resp = await fetch(`${api}/providers/apply-managed`, { method: 'POST' });
    if (!resp.ok) throw new Error('应用固定配置失败');
    await loadAll(false);
    return;
  }

  const endpoint = action === 'uninstall' ? '/providers/uninstall' : '/providers/install';
  const resp = await fetch(`${api}${endpoint}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: key })
  });
  if (!resp.ok) throw new Error(`${action} 失败`);
  await loadAll(false);
}

function bindDetailEditor() {
  detailConfigEl?.addEventListener('change', async (event) => {
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
    if (path === 'debug.model_url') {
      localStorage.setItem(modelStorageKey, nextValue);
      await loadMotionGroups(nextValue);
      const selected = modelCatalog.find((item) => String(item.url) === String(nextValue));
      const modelName = selected?.name || inferModelNameFromURL(nextValue);
      if (modelName) {
        if (!draftSettings.app || typeof draftSettings.app !== 'object') {
          draftSettings.app = {};
        }
        draftSettings.app.model_name = modelName;
        currentSettings = deepClone(draftSettings);
        await saveSettings(deepClone(draftSettings), `已切换模型目录：${modelName}`);
      } else {
        log('未能从模型路径推导 model_name，重启后可能不会保持该模型');
      }
    }
    if (path === 'debug.motion_group') {
      runModelAction('motion').catch((error) => log(String(error)));
    }
    if (!path.startsWith('debug.')) setDirty(true);
    renderSidebarText(currentSettings, currentProviderStatus);
    renderSidebarIndicators();
    renderDetailPanel();
  });

  detailConfigEl?.addEventListener('click', (event) => {
    const target = event.target;
    if (target?.dataset?.modelAction) {
      runModelAction(target.dataset.modelAction).catch((error) => log(String(error)));
      return;
    }
    if (target?.dataset?.providerAction) {
      runProviderAction(target.dataset.providerAction, target.dataset.providerKey).catch((error) => log(String(error)));
    }
  });

  detailApplyEl?.addEventListener('click', async () => {
    if (!detailDirty || !draftSettings) return;
    await saveSettings(deepClone(draftSettings), `已应用 ${moduleSchemas[selectedModule]?.title || '模块'} 配置改动`);
  });

  detailRevertEl?.addEventListener('click', () => {
    if (!currentSettings) return;
    draftSettings = deepClone(currentSettings);
    ensureDebugDraft();
    setDirty(false);
    renderDetailPanel();
  });
}

async function loadDiscoverOnly() {
  try {
    const resp = await fetch(`${api}/providers/discover`);
    if (!resp.ok) throw new Error('discover failed');
    const data = await resp.json();
    discoverItems = Array.isArray(data?.items) ? data.items : [];
  } catch (_) {
    discoverItems = [];
  }
}

async function loadModelCatalog() {
  try {
    const resp = await fetch(`${api}/runtime/model/list`, { cache: 'no-store' });
    if (!resp.ok) throw new Error(`model list failed: ${resp.status}`);
    const data = await resp.json();
    const items = Array.isArray(data?.items) ? data.items : [];
    modelCatalog = items
      .filter((item) => item && typeof item.model_url === 'string' && item.model_url.trim() !== '')
      .map((item) => ({
        name: String(item.name || ''),
        label: String(item.label || item.name || '未命名模型'),
        url: `${api}${String(item.model_url).trim()}`
      }));
  } catch (_) {
    modelCatalog = [];
  }
}

async function loadAll(appendLog = true) {
  const [settingsResp, providerResp] = await Promise.all([
    fetch(`${api}/settings/runtime`),
    fetch(`${api}/providers/status`)
  ]);
  if (!settingsResp.ok) throw new Error(`读取配置失败: ${settingsResp.status}`);
  if (!providerResp.ok) throw new Error(`读取 Provider 状态失败: ${providerResp.status}`);

  const settings = await settingsResp.json();
  currentProviderStatus = await providerResp.json();
  await loadModelCatalog();
  await loadDiscoverOnly();

  if (!detailDirty || !currentSettings) {
    currentSettings = settings;
    draftSettings = deepClone(settings);
    ensureDebugDraft();
    await loadMotionGroups(getValueByPath(draftSettings, 'debug.model_url'));
    setDirty(false);
  }

  renderSidebarText(currentSettings, currentProviderStatus);
  renderSidebarIndicators();
  renderDetailPanel();

  if (appendLog) log('控制台状态已刷新');
}

document.getElementById('refresh')?.addEventListener('click', () => {
  loadAll().catch((error) => log(String(error)));
});

bindSidebarModuleNav();
bindDetailEditor();
selectModule(selectedModule);
detectAPIBase()
  .then((base) => {
    api = base;
    return loadAll();
  })
  .catch((error) => log(String(error)));
setInterval(() => {
  loadAll(false).catch((error) => log(String(error)));
}, 5000);
