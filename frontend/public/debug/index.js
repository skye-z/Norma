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
  { key: 'piper', name: 'Piper', desc: '用于语音合成' }
];

const providerInstallPlan = {
  ollama: ['llm-ollama-windows-x64'],
  piper: ['tts-piper-windows-x64', 'tts-piper-voice-zh-cn-huayan-medium']
};

const moduleSchemas = {
  model: {
    title: '模型模块',
    summary: '切换 Live2D 模型，读取该模型动作组并手动触发动作和姿态。',
    fields: [
      { path: 'debug.model_url', label: '模型文件', type: 'model_select' },
      { path: 'debug.motion_index', label: '动作序号', type: 'number' }
    ]
  },
  input: {
    title: '输入模块',
    summary: '麦克风、唤醒词和前置音频策略（唤醒已内嵌）。',
    fields: [
      { path: 'audio.enable_wake_word', label: '启用唤醒词', type: 'boolean' },
      { path: 'audio.wake_mode', label: '唤醒模式', type: 'select', options: [{ value: 'wake_word_vad', label: '唤醒词+语音端点检测' }, { value: 'vad', label: '仅语音端点检测' }] },
      { path: 'audio.wake_keyword', label: '唤醒词', type: 'text' },
      { path: 'audio.mic_device_id', label: '麦克风设备', type: 'mic_device_select' }
    ]
  },
  asr: {
    title: '识别模块',
    summary: '语音识别配置（whisper 已内嵌）。',
    fields: [
      { path: 'asr.language', label: '识别语言', type: 'select', options: [{ value: 'zh', label: '中文 (zh)' }] },
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
      { path: 'rag.embed_endpoint', label: '嵌入接口地址', type: 'shared_endpoint' },
      { path: 'rag.embed_model', label: '嵌入模型', type: 'model_name_select' },
      { path: 'rag.vector_index_path', label: '向量索引路径', type: 'text' }
    ]
  },
  llm: {
    title: '对话模块',
    summary: '大模型推理配置。',
    fields: [
      { path: 'llm.provider', label: '大模型组件', type: 'select', options: [{ value: 'ollama', label: 'Ollama' }] },
      { path: 'llm.endpoint', label: '大模型接口地址', type: 'text' },
      { path: 'llm.model', label: '模型名称', type: 'model_name_select' },
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
let micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }];
let endpointModelOptions = [];
let endpointModelError = '';
const modelDownloadState = {
  asr_model: { job: null, last: null },
  wake_model: { job: null, last: null }
};

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
  draftSettings.audio.wake_provider = 'sherpa-onnx';
  draftSettings.asr.provider = 'whispercpp';
  draftSettings.asr.endpoint = '';
  draftSettings.asr.stream_endpoint = '';
  if (!draftSettings.tts.provider) draftSettings.tts.provider = 'piper';
  if (!draftSettings.llm.provider) draftSettings.llm.provider = 'ollama';
  if (!draftSettings.llm.endpoint) draftSettings.llm.endpoint = 'http://127.0.0.1:11434/v1';
  if (!draftSettings.llm.model) draftSettings.llm.model = '';
  if (!draftSettings.rag.embed_provider) draftSettings.rag.embed_provider = 'ollama';
  if (!draftSettings.rag.embed_endpoint) draftSettings.rag.embed_endpoint = draftSettings.llm.endpoint || 'http://127.0.0.1:11434/v1';
  if (!draftSettings.rag.embed_model) draftSettings.rag.embed_model = '';
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
      if (currentProviderStatus?.health?.wake?.status === 'ready') return 'green';
      if (currentProviderStatus?.health?.wake?.status === 'partial') return 'yellow';
      if (!currentSettings.audio?.wake_provider) return 'gray';
      if (!currentSettings.audio?.enable_wake_word) return 'red';
      const st = providerHealthStatus(stack.wake || 'sherpa-onnx');
      if (st === 'ready') return 'green';
      if (st === 'partial') return 'yellow';
      if (st === 'missing') return 'gray';
      return 'yellow';
    }
    case 'asr': {
      if (currentProviderStatus?.health?.asr?.status === 'ready') return 'green';
      if (currentProviderStatus?.health?.asr?.status === 'partial') return 'yellow';
      if (!currentSettings.asr?.provider) return 'gray';
      if (currentSettings.asr.provider === 'mock') return 'red';
      const st = providerHealthStatus(stack.asr || 'whispercpp');
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
  if (textAsrEl) textAsrEl.textContent = `内嵌 Whisper / ${settings?.asr?.language || 'zh'}`;
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

async function loadMicDeviceOptions() {
  try {
    if (!navigator?.mediaDevices?.enumerateDevices) {
      micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }];
      return;
    }
    const devices = await navigator.mediaDevices.enumerateDevices();
    const mics = devices.filter((item) => item.kind === 'audioinput');
    const opts = mics.map((item, index) => ({
      value: item.deviceId || 'default',
      label: item.label || `麦克风 ${index + 1}`
    }));
    micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }, ...opts];
  } catch (_) {
    micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }];
  }
}

async function loadEndpointModels(endpoint) {
  const base = String(endpoint || '').trim().replace(/\/+$/, '');
  if (!base) {
    endpointModelOptions = [];
    endpointModelError = '模型接口地址为空';
    return;
  }
  try {
    const modelsURL = /\/v1$/i.test(base) ? `${base}/models` : `${base}/v1/models`;
    const resp = await fetch(modelsURL, { cache: 'no-store' });
    if (!resp.ok) throw new Error(`接口返回 ${resp.status}`);
    const data = await resp.json();
    const list = Array.isArray(data?.data) ? data.data : [];
    endpointModelOptions = list
      .map((item) => ({ value: String(item?.id || '').trim(), label: String(item?.id || '').trim() }))
      .filter((item) => item.value);
    endpointModelError = endpointModelOptions.length ? '' : '模型接口可达，但返回空模型列表';
  } catch (_) {
    endpointModelOptions = [];
    endpointModelError = '模型接口不可达，请检查地址和服务状态';
  }
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
          </div>
        </div>
      `).join('')}
      <button data-provider-action="discover" type="button">重新发现已安装组件</button>
    </div>
  `;
}

function renderDownloadCard(module, title, desc, manualPath, state, detected = null) {
  const job = state?.job || null;
  const pct = job && Number.isFinite(job.percent) ? Math.max(0, Math.min(100, Math.round(job.percent))) : 0;
  const detectedReady = detected && detected.status === 'ready';
  const detectedPartial = detected && detected.status === 'partial';
  const statusText = detectedReady
    ? '已就绪'
    : detectedPartial
      ? '部分就绪'
      : job ? `${job.status}${job.message ? ` / ${job.message}` : ''}` : '未开始';
  const err = job?.error ? `<div class="field-hint-error">${escapeHtml(job.error)}</div>` : '';
  const detectedPaths = detected?.paths && typeof detected.paths === 'object'
    ? `<div class="event-detail">已识别文件:<br>${Object.entries(detected.paths).map(([key, value]) => `${escapeHtml(key)} = <code>${escapeHtml(value)}</code>`).join('<br>')}</div>`
    : '';
  return `
    <div class="download-card">
      <div class="download-head">
        <strong>${escapeHtml(title)}</strong>
        <span>${escapeHtml(statusText)}</span>
      </div>
      <div class="event-detail">${escapeHtml(desc)}</div>
      <div class="event-detail">手动放置目录: <code>${escapeHtml(manualPath)}</code></div>
      ${detectedPaths}
      <div class="progress-wrap"><div class="progress-bar" style="width:${pct}%;"></div></div>
      <div class="event-detail">进度: ${pct}%</div>
      <div class="provider-actions">
        <button data-download-module="${escapeHtml(module)}" data-download-action="start" type="button" ${job && job.status === 'running' ? 'disabled' : ''}>开始下载</button>
        <button data-download-module="${escapeHtml(module)}" data-download-action="refresh" type="button">刷新进度</button>
        <button data-download-module="${escapeHtml(module)}" data-download-action="open-dir" type="button">打开目录</button>
      </div>
      ${err}
    </div>
  `;
}

function renderEmbeddedDownloadCards() {
  if (selectedModule !== 'input' && selectedModule !== 'asr') return '';
  if (selectedModule === 'input') {
    if (currentProviderStatus?.health?.wake?.status === 'ready') return '';
    return renderDownloadCard('wake_model', '唤醒模型下载', '下载 Sherpa 唤醒模型与关键词文件（内嵌模式仅需模型资产）', 'resource/runtime/providers/wake/current', modelDownloadState.wake_model, currentProviderStatus?.health?.wake || null);
  }
  if (currentProviderStatus?.health?.asr?.status === 'ready') return '';
  return renderDownloadCard('asr_model', '识别模型下载', '下载 Whisper 基础模型（内嵌模式仅需模型文件）', 'resource/runtime/providers/asr/current', modelDownloadState.asr_model, currentProviderStatus?.health?.asr || null);
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
    if (field.type === 'mic_device_select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(micDeviceOptions, value || 'default')}</select></div>`;
    }
    if (field.type === 'shared_endpoint') {
      const endpoint = String(getValueByPath(draftSettings, 'llm.endpoint') || '');
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><input data-path="llm.endpoint" type="text" value="${escapeHtml(endpoint)}" /></div>`;
    }
    if (field.type === 'model_name_select') {
      const current = String(value || '');
      const options = endpointModelOptions.length
        ? endpointModelOptions
        : (current ? [{ value: current, label: `${current} (当前)` }] : []);
      const hint = endpointModelError ? `<div class="field-hint-error">${escapeHtml(endpointModelError)}</div>` : '';
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><div><div class="inline-actions"><select data-path="${escapeHtml(field.path)}">${optionListHTML(options, current)}</select><button data-model-action="refresh-models" type="button">刷新模型</button></div>${hint}</div></div>`;
    }
    const inputType = field.type === 'number' ? 'number' : 'text';
    return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><input data-path="${escapeHtml(field.path)}" type="${inputType}" value="${escapeHtml(value ?? '')}" /></div>`;
  }).join('');

  const selectedMotion = String(getValueByPath(draftSettings, 'debug.motion_group') || modelMotionGroups[0] || 'Idle');
  const motionButtons = selectedModule === 'model'
    ? `<div class="model-action-grid">${modelMotionGroups.map((group) => `<button data-motion-group="${escapeHtml(group)}" type="button" class="${selectedMotion === String(group) ? 'primary' : ''}">${escapeHtml(group)}</button>`).join('')}</div>`
    : '';

  const modelActions = selectedModule === 'model'
    ? `<div class="model-action-grid"><button data-model-action="reload" type="button">加载所选模型</button></div>${motionButtons}`
    : '';

  detailConfigEl.innerHTML = `${formHTML}${modelActions}${renderEmbeddedDownloadCards()}${renderProviderManageList()}`;
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
  if (kind === 'pose' || kind === 'mouth') return;
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

  const ids = providerInstallPlan[key] || [key];
  const endpoint = action === 'uninstall' ? '/providers/uninstall' : '/providers/install';
  const queue = action === 'uninstall' ? [...ids].reverse() : ids;
  for (const id of queue) {
    const resp = await fetch(`${api}${endpoint}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    if (!resp.ok) {
      const body = await resp.text().catch(() => '');
      throw new Error(`${action} 失败: ${id}${body ? ` -> ${body}` : ''}`);
    }
  }
  await loadAll(false);
}

async function startModelDownload(module) {
  const resp = await fetch(`${api}/providers/model-download/start`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ module })
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok || !data?.ok) {
    throw new Error(data?.error?.message || `启动下载失败: ${resp.status}`);
  }
  modelDownloadState[module] = { job: data.job || null, last: Date.now() };
}

async function refreshModelDownload(module) {
  const state = modelDownloadState[module];
  const jobID = state?.job?.id;
  if (!jobID) return;
  const resp = await fetch(`${api}/providers/model-download/status?job_id=${encodeURIComponent(jobID)}`);
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok || !data?.ok) {
    throw new Error(data?.error?.message || `刷新下载进度失败: ${resp.status}`);
  }
  modelDownloadState[module] = { job: data.job || null, last: Date.now() };
}

async function openModelTargetDir(module) {
  const resp = await fetch(`${api}/providers/model-download/open-dir`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ module })
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok || !data?.ok) {
    throw new Error(data?.error?.message || `打开目录失败: ${resp.status}`);
  }
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
    if (path === 'llm.endpoint') {
      setValueByPath(draftSettings, 'rag.embed_endpoint', String(nextValue || '').trim());
      await loadEndpointModels(String(nextValue || '').trim());
    }
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
    if (!path.startsWith('debug.')) setDirty(true);
    renderSidebarText(currentSettings, currentProviderStatus);
    renderSidebarIndicators();
    renderDetailPanel();
  });

  detailConfigEl?.addEventListener('click', (event) => {
    const target = event.target;
    if (target?.dataset?.motionGroup) {
      setValueByPath(draftSettings, 'debug.motion_group', String(target.dataset.motionGroup));
      renderDetailPanel();
      runModelAction('motion').catch((error) => log(String(error)));
      return;
    }
    if (target?.dataset?.modelAction === 'refresh-models') {
      const endpoint = String(getValueByPath(draftSettings, 'llm.endpoint') || '').trim();
      loadEndpointModels(endpoint)
        .then(() => {
          renderDetailPanel();
          log(`已刷新模型列表: ${endpoint || '-'}`);
        })
        .catch((error) => log(String(error)));
      return;
    }
    if (target?.dataset?.modelAction) {
      runModelAction(target.dataset.modelAction).catch((error) => log(String(error)));
      return;
    }
    if (target?.dataset?.providerAction) {
      runProviderAction(target.dataset.providerAction, target.dataset.providerKey).catch((error) => log(String(error)));
      return;
    }
    if (target?.dataset?.downloadAction) {
      const module = String(target.dataset.downloadModule || '').trim();
      if (!module) return;
      if (target.dataset.downloadAction === 'start') {
        startModelDownload(module)
          .then(() => {
            renderDetailPanel();
            log(`已启动下载任务: ${module}`);
          })
          .catch((error) => log(String(error)));
        return;
      }
      if (target.dataset.downloadAction === 'refresh') {
        refreshModelDownload(module)
          .then(() => renderDetailPanel())
          .catch((error) => log(String(error)));
        return;
      }
      if (target.dataset.downloadAction === 'open-dir') {
        openModelTargetDir(module)
          .then(() => log(`已打开目录: ${module}`))
          .catch((error) => log(String(error)));
        return;
      }
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

async function resolveRuntimeModelURL() {
  try {
    const resp = await fetch(`${api}/runtime/model/resolve`, { cache: 'no-store' });
    if (!resp.ok) return '';
    const data = await resp.json();
    if (!data?.ok || typeof data.model_url !== 'string' || !data.model_url.trim()) return '';
    return `${api}${String(data.model_url).trim()}`;
  } catch (_) {
    return '';
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
    setDirty(false);
  }

  await loadMicDeviceOptions();
  await loadEndpointModels(getValueByPath(draftSettings, 'llm.endpoint'));

  const resolvedModelURL = await resolveRuntimeModelURL();
  if (resolvedModelURL) {
    const current = String(getValueByPath(draftSettings, 'debug.model_url') || '').trim();
    if (!current || !modelCatalog.some((item) => String(item.url) === current)) {
      setValueByPath(draftSettings, 'debug.model_url', resolvedModelURL);
    }
  }
  await loadMotionGroups(getValueByPath(draftSettings, 'debug.model_url'));

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
