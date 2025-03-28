import './style.css';
import * as PIXI from 'pixi.js';

window.PIXI = PIXI;

let apiBase = window.location.origin && window.location.origin.startsWith('http')
  ? window.location.origin
  : 'http://127.0.0.1:8090';
const fallbackApiBase = 'http://127.0.0.1:8090';

async function detectAPIBase() {
  const candidates = [apiBase, fallbackApiBase].filter((value, index, arr) => value && arr.indexOf(value) === index);
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
  return fallbackApiBase;
}
async function resolveModelMeta() {
  let localPreferred = '';
  try {
    localPreferred = String(localStorage.getItem('norma.debug.model_url') || '').trim();
  } catch (_) {}

  const resp = await fetch(`${apiBase}/runtime/model/resolve`, { cache: 'no-store' });
  if (!resp.ok) {
    throw new Error(`/runtime/model/resolve -> ${resp.status}`);
  }
  const data = await resp.json();
  if (!data?.ok || typeof data.model_url !== 'string' || !data.model_url.trim()) {
    throw new Error('runtime/model/resolve 返回了无效模型路径');
  }
  const modelURL = localPreferred || `${apiBase}${data.model_url}`;
  return {
    modelURL,
    modelName: String(data.model_name || ''),
    modelMapping: data.mapping && typeof data.mapping === 'object' ? data.mapping : { motions: {}, expressions: {} }
  };
}

function normalizeModelURL(input) {
  const raw = String(input || '').trim();
  if (!raw) return '';
  if (raw.startsWith('http://') || raw.startsWith('https://')) return raw;
  if (raw.startsWith('/')) return `${apiBase}${raw}`;
  return `${apiBase}/${raw}`;
}

function siblingCoreURL(modelURL) {
  const _ = modelURL;
  return '/live2dcubismcore.min.js';
}

const params = new URLSearchParams(window.location.search);
const mode = params.get('debug') === '1'
  ? 'debug'
  : params.get('settings') === '1'
    ? 'settings'
    : params.get('onboarding') === '1'
      ? 'onboarding'
      : 'companion';

function mount(html) {
  const app = document.querySelector('#app');
  if (app) app.innerHTML = html;
}

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

async function fetchJSON(path, options) {
  const resp = await fetch(`${apiBase}${path}`, options);
  if (!resp.ok) throw new Error(`${path} -> ${resp.status}`);
  return resp.json();
}

async function postJSON(path, body) {
  const resp = await fetch(`${apiBase}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined
  });
  if (!resp.ok) throw new Error(`${path} -> ${resp.status}`);
  return resp.json();
}

async function publish(name, payload, meta = {}) {
  const sessionID = meta.sessionID || `wails-${mode}`;
  const traceID = meta.traceID || `wails-${mode}`;
  return postJSON('/runtime/publish', {
    name,
    session_id: sessionID,
    trace_id: traceID,
    payload
  });
}

async function waitForApiReady(timeoutMs = 10000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const resp = await fetch(`${apiBase}/healthz`, { cache: 'no-store' });
      if (resp.ok) return true;
    } catch (_) {}
    await sleep(300);
  }
  return false;
}

function createRuntimeEventSource(name, onPayload, onError) {
  const parsed = new URL(apiBase);
  parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsURL = `${parsed.origin}/runtime/events/ws?name=${encodeURIComponent(name)}`;
  let socket = null;
  let reconnectTimer = null;
  let closed = false;

  const connect = () => {
    if (closed) return;
    socket = new WebSocket(wsURL);
    socket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        onPayload(data.payload, data);
      } catch (_) {}
    };
    socket.onerror = () => {
      try { socket.close(); } catch (_) {}
    };
    socket.onclose = () => {
      if (closed) return;
      if (typeof onError === 'function') onError(name);
      reconnectTimer = window.setTimeout(connect, 500);
    };
  };

  connect();

  return {
    close() {
      closed = true;
      if (reconnectTimer) {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      if (socket && (socket.readyState === WebSocket.CONNECTING || socket.readyState === WebSocket.OPEN)) {
        try { socket.close(); } catch (_) {}
      }
    }
  };
}

function navTabs(active) {
  const tabs = [
    ['http://127.0.0.1:8090/static/debug/index.html', '控制台', 'home'],
    ['?settings=1', '设置', 'settings'],
    ['http://127.0.0.1:8090/static/debug/index.html', '控制台首页', 'external']
  ];
  return tabs.map(([href, label, key]) => `<a class="${key === active ? 'active' : ''}" href="${href}">${label}</a>`).join('');
}

function renderConsoleShell(active, heroTag, heroTitle, heroText, actions, body) {
  document.body.classList.add('console-mode');
  return `
    <main class="console-shell">
      <header class="console-topbar">
        <div class="console-brand">
          <span class="console-brand-mark"></span>
          <div>
            <div class="console-brand-title">NORMA / CONTROL SURFACE</div>
            <div class="console-brand-sub">桌面伴侣运行时控制台</div>
          </div>
        </div>
        <nav class="console-nav">${navTabs(active)}</nav>
        <div class="console-barcode"><span></span><span></span><span></span><span></span><span></span><span></span></div>
      </header>
      <section class="console-hero">
        <div class="console-hero-tag">${heroTag}</div>
        <h1>${heroTitle}</h1>
        <p>${heroText}</p>
        <div class="console-actions">${actions}</div>
      </section>
      ${body}
    </main>
  `;
}

function statePill(status) {
  const cls = status === 'ready' ? 'console-pill' : status === 'partial' ? 'console-pill warn' : 'console-pill danger';
  const label = status === 'ready' ? '就绪' : status === 'partial' ? '部分就绪' : '缺失';
  return `<span class="${cls}">${label}</span>`;
}

function writeLog(target, message) {
  if (!target) return;
  const line = `[${new Date().toLocaleTimeString()}] ${message}`;
  target.textContent = `${line}\n${target.textContent}`;
}

function renderSettingsPage() {
  mount(renderConsoleShell(
    'settings',
    'Managed Providers',
    '运行时设置',
    '查看当前 provider 栈、安装状态和就绪度，并在这里执行初始化、应用受管 Provider 和导出审计。',
    '<button id="refresh-settings" class="primary">刷新状态</button><button id="init-providers">初始化 Provider 目录</button><button id="apply-managed">应用受管 Provider</button>',
    `
      <section class="console-grid console-grid-settings">
        <aside class="console-panel">
          <div class="console-panel-head"><h2>当前栈</h2><span class="console-mini">ACTIVE STACK</span></div>
          <div class="console-panel-body console-stack" id="stack"></div>
        </aside>
        <section class="console-panel">
          <div class="console-panel-head"><h2>Provider 就绪度</h2><span class="console-mini">READY / PARTIAL / MISSING</span></div>
          <div class="console-panel-body console-stack" id="health"></div>
        </section>
        <aside class="console-panel">
          <div class="console-panel-head"><h2>操作日志</h2><span class="console-mini">ACTIONS</span></div>
          <div class="console-panel-body console-stack">
            <div class="console-actions">
              <button id="export-readiness">导出就绪检查</button>
              <button id="open-readiness-dir">打开导出目录</button>
            </div>
            <pre id="settings-log" class="console-terminal"></pre>
          </div>
        </aside>
      </section>
    `
  ));

  const stackEl = document.getElementById('stack');
  const healthEl = document.getElementById('health');
  const logEl = document.getElementById('settings-log');

  async function refreshStatus() {
    const data = await fetchJSON('/providers/status');
    stackEl.innerHTML = Object.entries(data.stack || {}).map(([name, value]) => `
      <div class="console-stat-line"><strong>${name}</strong><span>${value || '-'}</span></div>
    `).join('') || '<div class="console-foot">暂无运行栈信息</div>';

    healthEl.innerHTML = Object.entries(data.health || {}).map(([name, item]) => `
      <div class="console-card">
        <div class="console-stat-line"><strong>${name}</strong>${statePill(item.status)}</div>
        <div class="console-foot">找到：${(item.found || []).join('，') || '无'}</div>
        <div class="console-foot">缺失：${(item.missing || []).join('，') || '无'}</div>
        ${item?.paths && typeof item.paths === 'object' && Object.keys(item.paths).length ? `<div class="console-foot">路径：${Object.entries(item.paths).map(([key, value]) => `${key}=${String(value).replaceAll('<', '&lt;')}`).join('<br>')}</div>` : ''}
      </div>
    `).join('') || '<div class="console-foot">暂无 provider 信息</div>';

    writeLog(logEl, `已刷新 Provider 状态，根目录：${data.root}`);
  }

  document.getElementById('refresh-settings')?.addEventListener('click', () => refreshStatus().catch((err) => writeLog(logEl, String(err))));
  document.getElementById('init-providers')?.addEventListener('click', async () => {
    await postJSON('/providers/init');
    writeLog(logEl, '已初始化 Provider 目录');
    await refreshStatus();
  });
  document.getElementById('apply-managed')?.addEventListener('click', async () => {
    await postJSON('/providers/apply-managed');
    writeLog(logEl, '已应用受管 Provider 到当前配置');
    await refreshStatus();
  });
  document.getElementById('export-readiness')?.addEventListener('click', async () => {
    const data = await postJSON('/providers/export-readiness');
    writeLog(logEl, `已导出就绪检查：${data.path}`);
  });
  document.getElementById('open-readiness-dir')?.addEventListener('click', async () => {
    await postJSON('/providers/open-readiness-dir');
    writeLog(logEl, '已请求打开导出目录');
  });

  refreshStatus().catch((err) => writeLog(logEl, String(err)));
}

function renderOnboardingPage() {
  mount(renderConsoleShell(
    'onboarding',
    'First Run Check',
    '首启向导',
    '用于首装时检查模型、Provider、配置文件和设备脚本，快速识别是否能直接进入桌面伴侣模式。',
    '<button id="refresh-onboarding" class="primary">重新检测</button><a class="console-button" href="?settings=1">转到设置</a>',
    `
      <section class="console-grid console-grid-onboarding">
        <aside class="console-panel">
          <div class="console-panel-head"><h2>总体状态</h2><span class="console-mini">SETUP READY</span></div>
          <div class="console-panel-body console-stack">
            <div id="overall"></div>
            <div class="console-card"><div class="console-label">建议动作</div><div id="suggestions" class="console-stack"></div></div>
          </div>
        </aside>
        <section class="console-panel">
          <div class="console-panel-head"><h2>检查项</h2><span class="console-mini">CHECKLIST</span></div>
          <div class="console-panel-body console-stack" id="checks"></div>
        </section>
        <aside class="console-panel">
          <div class="console-panel-head"><h2>回退方案</h2><span class="console-mini">FALLBACK MODES</span></div>
          <div class="console-panel-body console-stack" id="fallbacks"></div>
        </aside>
      </section>
    `
  ));

  const overallEl = document.getElementById('overall');
  const suggestionsEl = document.getElementById('suggestions');
  const checksEl = document.getElementById('checks');
  const fallbacksEl = document.getElementById('fallbacks');

  async function refreshWizard() {
    const data = await fetchJSON('/setup/wizard');
    overallEl.innerHTML = `
      <div class="console-card">
        <div class="console-stat-line"><strong>首启状态</strong>${statePill(data.ready ? 'ready' : 'partial')}</div>
        <div class="console-foot">${data.ready ? '当前配置已可进入桌面伴侣模式。' : '仍有缺项，建议先补齐关键 Provider 或模型资源。'}</div>
      </div>
    `;
    suggestionsEl.innerHTML = (data.suggestions || []).map((item) => `<div class="console-foot">${item}</div>`).join('') || '<div class="console-foot">暂无建议</div>';
    checksEl.innerHTML = (data.checks || []).map((item) => `
      <div class="console-card">
        <div class="console-stat-line"><strong>${item.name}</strong>${statePill(item.status)}</div>
        <div class="console-foot">${item.detail || '-'}</div>
        <div class="console-foot">建议：${item.hint || '-'}</div>
      </div>
    `).join('') || '<div class="console-foot">暂无检查项</div>';
    fallbacksEl.innerHTML = (data.fallback_modes || []).map((item) => `<div class="console-card"><div class="console-foot">${item}</div></div>`).join('') || '<div class="console-foot">暂无回退方案</div>';
  }

  document.getElementById('refresh-onboarding')?.addEventListener('click', () => refreshWizard().catch(console.error));
  refreshWizard().catch(console.error);
}

function renderCompanionPage() {
  document.body.classList.remove('console-mode');
  document.addEventListener('contextmenu', (event) => {
    event.preventDefault();
  }, { capture: true });
  mount(`
    <main class="companion-shell">
      <div id="stage-status" class="stage-status" data-kind="info">正在加载 Live2D 模型...</div>
      <button id="mic-fab" class="mic-fab" type="button" aria-label="语音输入" aria-pressed="false" title="点击开始语音输入">
        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path d="M12 15a3 3 0 0 0 3-3V7a3 3 0 1 0-6 0v5a3 3 0 0 0 3 3Z"></path>
          <path d="M18 12a1 1 0 1 0-2 0 4 4 0 1 1-8 0 1 1 0 1 0-2 0 6 6 0 0 0 5 5.91V21H9a1 1 0 1 0 0 2h6a1 1 0 1 0 0-2h-2v-3.09A6 6 0 0 0 18 12Z"></path>
        </svg>
      </button>
      <div id="stage" class="stage wails-drag"></div>
    </main>
  `);
}

async function ensureCore(coreURL) {
  if (window.Live2DCubismCore) return;
  await new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = coreURL;
    script.async = true;
    script.onload = resolve;
    script.onerror = () => reject(new Error('live2d core load failed'));
    document.head.appendChild(script);
  });
}

function writeParam(core, id, value) {
  if (typeof core.setParameterValueById === 'function') {
    core.setParameterValueById(id, value);
    return;
  }
  if (typeof core.addParameterValueById === 'function') {
    const current = typeof core.getParameterValueById === 'function' ? core.getParameterValueById(id) : 0;
    core.addParameterValueById(id, value - current);
  }
}

function normalizeMotionName(payload) {
  const aliases = { thinking: 'Tap', tapbody: 'Tap@Body', flickbody: 'Flick@Body', idle: 'Idle' };
  const raw = typeof payload === 'string' ? payload : String(payload?.group || payload?.motion || payload?.name || 'Idle');
  const key = raw.replace(/[^a-zA-Z]/g, '').toLowerCase();
  return aliases[key] || raw;
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

async function initCompanion() {
  renderCompanionPage();

  const stage = document.getElementById('stage');
  const statusEl = document.getElementById('stage-status');
  const micFabEl = document.getElementById('mic-fab');
  const state = {
    app: null,
    model: null,
    Live2DModel: null,
    mouthCurrent: 0,
    mouthTarget: 0,
    mouthForm: 0,
    targetX: 0,
    targetY: 0,
    focusX: 0,
    focusY: 0,
    motionIntensity: 0.45,
    eventSources: [],
    baseX: 0,
    baseY: 0,
    sessionID: '',
    traceID: '',
    resizeObserver: null,
    layoutHandler: null,
    speechRecognition: null,
    useSpeechFallback: false,
    speechListening: false,
    idleTimer: null,
    mediaStream: null,
    mediaContext: null,
    mediaProcessor: null,
    captureMode: '',
    sessionMode: '',
    playbackContext: null,
    playbackQueue: Promise.resolve(),
    audioFormat: { sampleRate: 22050, channels: 1, encoding: 'pcm_s16le' },
    playbackState: {
      started: false,
      pending: 0,
      sessionID: '',
      traceID: '',
      interrupted: false,
      nextStartAt: 0
    },
    motionState: {
      currentGroup: 'Idle',
      priority: 0,
      cooldownUntil: 0,
      idleTimer: null,
      expressionTimer: null,
      paramTargets: {},
      paramValues: {},
      paramBlend: 0.18
    },
    live2dCatalog: {
      motionGroups: new Map(),
      expressions: new Map(),
      canonicalMotions: new Map(),
      modelMappings: { motions: {}, expressions: {} }
    },
    runtimeHints: {
      keepMicCaptureAlive: false,
      needRealAECNS: false
    }
  };

  function idleAfterMotion(ms = 1200) {
    if (!state.model) return;
    if (state.motionState.idleTimer) {
      window.clearTimeout(state.motionState.idleTimer);
      state.motionState.idleTimer = null;
    }
    state.motionState.idleTimer = window.setTimeout(async () => {
      if (!state.model) return;
      try {
        await state.model.motion(resolveMotionGroup('idle'), 0, 1);
      } catch (_) {}
    }, ms);
  }

  function scheduleExpressionReset(ms = 1200) {
    if (state.motionState.expressionTimer) {
      window.clearTimeout(state.motionState.expressionTimer);
      state.motionState.expressionTimer = null;
    }
    state.motionState.expressionTimer = window.setTimeout(() => {
      state.motionState.paramTargets = {};
    }, ms);
  }

  function normalizeKey(value) {
    return String(value || '').trim().toLowerCase();
  }

  function buildLive2DCatalog(modelMappings = { motions: {}, expressions: {} }) {
    const catalog = {
      motionGroups: new Map(),
      expressions: new Map(),
      canonicalMotions: new Map(),
      modelMappings
    };
    const settings = state.model?.internalModel?.settings;
    const defs = settings?.motions || settings?.Motions || {};
    Object.keys(defs || {}).forEach((group) => {
      catalog.motionGroups.set(normalizeKey(group), group);
    });
    const loadedGroups = state.model?.internalModel?.motionManager?.motionGroups || {};
    Object.keys(loadedGroups || {}).forEach((group) => {
      if (!catalog.motionGroups.has(normalizeKey(group))) {
        catalog.motionGroups.set(normalizeKey(group), group);
      }
    });
    const expressions = settings?.expressions || settings?.Expressions || [];
    expressions.forEach((expr, index) => {
      const name = normalizeKey(expr?.Name || expr?.name || `expr-${index}`);
      if (name) catalog.expressions.set(name, expr?.Name || expr?.name || index);
    });
    const expressionManager = state.model?.internalModel?.motionManager?.expressionManager;
    const definitions = expressionManager?.definitions || [];
    definitions.forEach((expr, index) => {
      const name = normalizeKey(expr?.Name || expr?.name || `expr-${index}`);
      if (name && !catalog.expressions.has(name)) {
        catalog.expressions.set(name, expr?.Name || expr?.name || index);
      }
    });

    const aliasGroups = {
      idle: ['idle', 'idle_01', 'idle01', 'main_idle'],
      thinking: ['thinking', 'think', 'idle_talk', 'tapbody', 'tap'],
      tap: ['tap', 'tapbody', 'touch_head', 'touch_special'],
      flick: ['flick', 'shake', 'damage', 'angry']
    };
    Object.entries(modelMappings?.motions || {}).forEach(([canonical, value]) => {
      const aliases = Array.isArray(value) ? value : [value];
      aliasGroups[normalizeKey(canonical)] = aliases.map((item) => String(item || ''));
    });
    Object.entries(aliasGroups).forEach(([canonical, aliases]) => {
      for (const alias of aliases) {
        const resolved = catalog.motionGroups.get(normalizeKey(alias));
        if (resolved) {
          catalog.canonicalMotions.set(canonical, resolved);
          break;
        }
      }
    });

    if (!catalog.canonicalMotions.has('idle')) catalog.canonicalMotions.set('idle', 'Idle');
    if (!catalog.canonicalMotions.has('tap')) catalog.canonicalMotions.set('tap', 'Tap');
    if (!catalog.canonicalMotions.has('flick')) catalog.canonicalMotions.set('flick', 'Flick');
    if (!catalog.canonicalMotions.has('thinking')) catalog.canonicalMotions.set('thinking', catalog.canonicalMotions.get('tap') || 'Tap');

    state.live2dCatalog = catalog;
  }

  function resolveMotionGroup(group) {
    const raw = String(group || 'Idle');
    const normalized = normalizeKey(raw);
    return state.live2dCatalog.canonicalMotions.get(normalized) || state.live2dCatalog.motionGroups.get(normalized) || raw;
  }

  function resolveExpressionName(name) {
    const normalized = normalizeKey(name);
    const mapped = state.live2dCatalog.modelMappings?.expressions?.[normalized];
    if (mapped) {
      const resolvedMapped = state.live2dCatalog.expressions.get(normalizeKey(mapped));
      if (resolvedMapped !== undefined) return resolvedMapped;
    }
    return state.live2dCatalog.expressions.get(normalized) || '';
  }

  async function playMotionWithPolicy(payload) {
    if (!state.model) return;
    const group = resolveMotionGroup(normalizeMotionName(payload));
    const index = Number.isInteger(payload?.index) ? payload.index : 0;
    const priority = Number.isFinite(Number(payload?.priority)) ? Number(payload.priority) : 50;
    const cooldownMs = Number.isFinite(Number(payload?.cooldown_ms)) ? Number(payload.cooldown_ms) : 0;
    const interruptible = payload?.interruptible !== false;
    const force = payload?.force === true;
    const now = Date.now();
    const current = state.motionState;
    if (!force) {
      if (!interruptible && current.cooldownUntil > now && priority <= current.priority) return;
      if (current.cooldownUntil > now && priority < current.priority) return;
      if (current.currentGroup === group && current.cooldownUntil > now) return;
    }
    current.currentGroup = String(group || 'Idle');
    current.priority = priority;
    current.cooldownUntil = now + Math.max(0, cooldownMs);
    try {
      await state.model.motion(group, index, 3);
      if (String(group).toLowerCase() !== 'idle') {
        idleAfterMotion(Number.isFinite(Number(payload?.idle_after_ms)) ? Number(payload.idle_after_ms) : 1200);
      }
    } catch (_) {
      for (const fallback of [resolveMotionGroup('tap'), resolveMotionGroup('flick'), resolveMotionGroup('idle')]) {
        try {
          await state.model.motion(fallback, 0, 3);
          break;
        } catch (_) {}
      }
    }
  }

  async function playNativeExpression(payload) {
    if (!state.model || !payload || typeof payload !== 'object') return false;
    const resolved = resolveExpressionName(payload.name);
    if (!resolved || typeof state.model.expression !== 'function') return false;
    try {
      const ok = await state.model.expression(resolved);
      if (ok) {
        scheduleExpressionReset(Number.isFinite(Number(payload.hold_ms)) ? Number(payload.hold_ms) : 1200);
        return true;
      }
    } catch (_) {}
    return false;
  }

  function debugCatalogSummary() {
    return {
      motions: Array.from(state.live2dCatalog.motionGroups.values()),
      expressions: Array.from(state.live2dCatalog.expressions.keys()),
      canonical: Object.fromEntries(state.live2dCatalog.canonicalMotions.entries()),
      mappings: state.live2dCatalog.modelMappings
    };
  }

  function setMicListening(listening) {
    state.speechListening = !!listening;
    if (!micFabEl) return;
    micFabEl.classList.toggle('is-listening', !!listening);
    micFabEl.setAttribute('aria-pressed', listening ? 'true' : 'false');
    micFabEl.title = listening ? '点击停止语音输入' : '点击开始语音输入';
  }

  function setupSpeechRecognition() {
    const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!SR) return null;
    const recognition = new SR();
    recognition.lang = 'zh-CN';
    recognition.continuous = true;
    recognition.interimResults = true;

    recognition.onstart = () => {
      setMicListening(true);
      setStatus('语音输入已启动，请开始说话...', 'info');
    };

    recognition.onresult = (event) => {
      let interimText = '';
      let finalText = '';
      for (let i = event.resultIndex; i < event.results.length; i += 1) {
        const transcript = String(event.results[i]?.[0]?.transcript || '').trim();
        if (!transcript) continue;
        if (event.results[i].isFinal) {
          finalText += transcript;
        } else {
          interimText += transcript;
        }
      }
      if (interimText) {
        setStatus(`识别中：${interimText}`, 'info');
        if (state.useSpeechFallback) {
          publish('asr.partial', interimText, { sessionID: state.sessionID, traceID: state.traceID }).catch(() => undefined);
        }
      }
      if (finalText) {
        setStatus(`已识别：${finalText}`, 'ready', true);
        if (state.useSpeechFallback) {
          publish('asr.final', finalText, { sessionID: state.sessionID, traceID: state.traceID }).catch(() => undefined);
        }
      }
    };

    recognition.onerror = (event) => {
      setMicListening(false);
      setStatus(`语音输入失败：${event?.error || 'unknown'}`, 'error');
    };

    recognition.onend = () => {
      const wasListening = state.speechListening;
      setMicListening(false);
      if (wasListening) setStatus('语音输入已结束', 'info', true);
    };

    return recognition;
  }

  function downsampleBuffer(input, inputRate, targetRate) {
    if (!input || !input.length) return new Float32Array(0);
    if (!inputRate || inputRate === targetRate) return input;
    const ratio = inputRate / targetRate;
    const length = Math.max(1, Math.round(input.length / ratio));
    const output = new Float32Array(length);
    for (let i = 0; i < length; i += 1) {
      const idx = Math.min(input.length - 1, Math.round(i * ratio));
      output[i] = input[idx];
    }
    return output;
  }

  function pcm16ToFloat32(buffer) {
    const view = new DataView(buffer);
    const out = new Float32Array(buffer.byteLength / 2);
    for (let i = 0; i < out.length; i += 1) {
      out[i] = view.getInt16(i * 2, true) / 32768;
    }
    return out;
  }

  async function sendMicFrame(frame, eof = false) {
    if (!state.sessionID || state.captureMode !== 'push') return;
    await postJSON('/conversation/mic/frame', {
      session_id: state.sessionID,
      trace_id: state.traceID,
      frames: Array.from(frame || []),
      eof
    }).catch(() => undefined);
  }

  async function stopMediaCapture(eof = true) {
    if (state.mediaProcessor) {
      try { state.mediaProcessor.disconnect(); } catch (_) {}
      state.mediaProcessor.onaudioprocess = null;
      state.mediaProcessor = null;
    }
    if (state.mediaStream) {
      state.mediaStream.getTracks().forEach((track) => {
        try { track.stop(); } catch (_) {}
      });
      state.mediaStream = null;
    }
    if (state.mediaContext) {
      try { await state.mediaContext.close(); } catch (_) {}
      state.mediaContext = null;
    }
    if (eof) await sendMicFrame([], true);
  }

  async function startMediaCapture() {
    if (state.mediaStream && state.mediaContext && state.mediaProcessor) return;
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const AudioCtx = window.AudioContext || window.webkitAudioContext;
    if (!AudioCtx) throw new Error('当前环境不支持 Web Audio');
    const audioCtx = new AudioCtx();
    const source = audioCtx.createMediaStreamSource(stream);
    const processor = audioCtx.createScriptProcessor(2048, 1, 1);
    processor.onaudioprocess = (event) => {
      if (!state.speechListening) return;
      const input = event.inputBuffer.getChannelData(0);
      const downsampled = downsampleBuffer(input, audioCtx.sampleRate, 16000);
      sendMicFrame(downsampled, false).catch(() => undefined);
    };
    source.connect(processor);
    processor.connect(audioCtx.destination);
    state.mediaStream = stream;
    state.mediaContext = audioCtx;
    state.mediaProcessor = processor;
  }

  async function playPCMChunk(base64Chunk) {
    if (window.__NORMA_NATIVE_AUDIO__) return null;
    if (!base64Chunk) return;
    if (!state.playbackContext) {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!AudioCtx) throw new Error('当前环境不支持音频播放');
      state.playbackContext = new AudioCtx({ sampleRate: state.audioFormat.sampleRate || 22050 });
    }
    const binary = atob(base64Chunk);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
    const samples = pcm16ToFloat32(bytes.buffer);
    const buffer = state.playbackContext.createBuffer(state.audioFormat.channels || 1, samples.length, state.audioFormat.sampleRate || 22050);
    buffer.copyToChannel(samples, 0);
    const node = state.playbackContext.createBufferSource();
    node.buffer = buffer;
    node.connect(state.playbackContext.destination);
    const playback = state.playbackState;
    const now = state.playbackContext.currentTime;
    const startAt = Math.max(playback.nextStartAt || now, now + 0.01);
    playback.nextStartAt = startAt + buffer.duration;
    await new Promise((resolve) => {
      node.onended = resolve;
      node.start(startAt);
    });
    return samples;
  }

  function resetPlaybackState() {
    state.playbackState.started = false;
    state.playbackState.pending = 0;
    state.playbackState.sessionID = '';
    state.playbackState.traceID = '';
    state.playbackState.interrupted = false;
    state.playbackState.nextStartAt = 0;
  }

  function markPlaybackStart(sessionID, traceID) {
    if (state.playbackState.started) return;
    state.playbackState.started = true;
    state.playbackState.sessionID = sessionID;
    state.playbackState.traceID = traceID;
    publish('audio.play.start', {
      source: 'web-audio',
      sample_rate: state.audioFormat.sampleRate,
      channels: state.audioFormat.channels,
      encoding: state.audioFormat.encoding
    }, { sessionID, traceID }).catch(() => undefined);
  }

  function finishPlaybackIfIdle(reason = 'done') {
    const playback = state.playbackState;
    if (!playback.started || playback.pending > 0) return;
    const sessionID = playback.sessionID || state.sessionID || '';
    const traceID = playback.traceID || state.traceID || sessionID || '';
    publish('audio.play.end', { source: 'web-audio', reason }, { sessionID, traceID }).catch(() => undefined);
    resetPlaybackState();
  }

  function computeChunkLevel(samples) {
    if (!samples || !samples.length) return 0;
    let sum = 0;
    for (let i = 0; i < samples.length; i += 1) {
      const v = samples[i];
      sum += v * v;
    }
    const rms = Math.sqrt(sum / samples.length);
    return Math.max(0, Math.min(1.15, rms * 6.5));
  }

  function driveMouthFromSamples(samples, sessionID, traceID) {
    const level = computeChunkLevel(samples);
    state.mouthTarget = level;
    publish('live2d.mouth', { level }, { sessionID, traceID }).catch(() => undefined);
  }

  async function waitForStageReady(minWidth = 120, minHeight = 180, timeoutMs = 1800) {
    if (!stage) throw new Error('stage container missing');
    const start = Date.now();
    let stableFrames = 0;
    let lastWidth = 0;
    let lastHeight = 0;

    while (Date.now() - start < timeoutMs) {
      await new Promise((resolve) => window.requestAnimationFrame(() => resolve()));
      const width = stage.clientWidth;
      const height = stage.clientHeight;
      const visible = stage.getBoundingClientRect();
      const valid = width >= minWidth && height >= minHeight && visible.width >= minWidth && visible.height >= minHeight;
      const stable = Math.abs(width - lastWidth) <= 1 && Math.abs(height - lastHeight) <= 1;

      if (valid && stable) {
        stableFrames += 1;
        if (stableFrames >= 2) return true;
      } else if (valid) {
        stableFrames = 1;
      } else {
        stableFrames = 0;
      }

      lastWidth = width;
      lastHeight = height;
      await sleep(60);
    }

    return false;
  }

  function setStatus(text, kind = 'info', autoHide = false) {
    if (!statusEl) return;
    statusEl.textContent = text;
    statusEl.dataset.kind = kind;
    statusEl.style.display = text ? 'block' : 'none';
    if (autoHide && text) {
      window.setTimeout(() => {
        if (statusEl.textContent === text) statusEl.style.display = 'none';
      }, 1600);
    }
  }

  function closeEvents() {
    state.eventSources.forEach((source) => {
      try { source.close(); } catch (_) {}
    });
    state.eventSources = [];
  }

  function destroy() {
    closeEvents();
    if (state.motionState.idleTimer) {
      window.clearTimeout(state.motionState.idleTimer);
      state.motionState.idleTimer = null;
    }
    if (state.motionState.expressionTimer) {
      window.clearTimeout(state.motionState.expressionTimer);
      state.motionState.expressionTimer = null;
    }
    if (state.speechRecognition && state.speechListening) {
      try { state.speechRecognition.stop(); } catch (_) {}
    }
    stopMediaCapture(false).catch(() => undefined);
    resetPlaybackState();
    state.captureMode = '';
    state.speechRecognition = null;
    setMicListening(false);
    if (state.resizeObserver) {
      try { state.resizeObserver.disconnect(); } catch (_) {}
      state.resizeObserver = null;
    }
    if (state.layoutHandler) {
      window.removeEventListener('resize', state.layoutHandler);
      state.layoutHandler = null;
    }
    if (state.app) {
      try {
        state.app.destroy(true, { children: true, texture: true, baseTexture: true });
      } catch (_) {}
      state.app = null;
    }
    state.model = null;
    if (stage) stage.innerHTML = '';
  }

  state.speechRecognition = setupSpeechRecognition();
  micFabEl?.addEventListener('click', () => {
    try {
      if (state.speechListening) {
        postJSON('/conversation/mic/interrupt', {
          session_id: state.sessionID,
          trace_id: state.traceID,
          reason: 'mic_toggle_stop'
        }).catch(() => undefined);
        setMicListening(false);
        setStatus('语音输入已停止', 'info', true);
        if (state.playbackState.started) {
          state.playbackState.pending = 0;
          state.playbackState.interrupted = true;
          finishPlaybackIfIdle('interrupt');
        }
        if (!window.__NORMA_KEEP_CAPTURE_ALIVE__ && !state.runtimeHints.keepMicCaptureAlive) {
          stopMediaCapture(true).catch(() => undefined);
        }
        if (state.useSpeechFallback) {
          state.speechRecognition.stop();
        }
        state.captureMode = '';
        state.sessionMode = '';
        state.sessionID = '';
        state.traceID = '';
      } else {
        postJSON('/conversation/mic/start', { mode: 'manual' }).then((started) => {
          state.sessionID = String(started?.session_id || '');
          state.traceID = String(started?.trace_id || started?.session_id || '');
          state.captureMode = String(started?.capture || '');
          state.sessionMode = String(started?.mode || 'manual');
          setMicListening(true);
          if (state.captureMode === 'native') {
            state.useSpeechFallback = false;
            setStatus(state.sessionMode === 'manual' ? '语音输入已启动（手动直通识别）' : '语音输入已启动（桌面原生采集）', 'info');
            return;
          }
          startMediaCapture().then(() => {
            state.useSpeechFallback = false;
            setStatus(state.sessionMode === 'manual' ? '语音输入已启动（手动直通识别）' : '语音输入已启动（真实麦克风）', 'info');
          }).catch(async () => {
            if (!state.speechRecognition) throw new Error('native capture unavailable and no browser fallback recognizer');
            state.useSpeechFallback = true;
            setStatus('语音输入已启动（浏览器识别 fallback）', 'info');
            state.speechRecognition.start();
          });
        }).catch(async () => {
          if (!state.speechRecognition) throw new Error('conversation start failed and no browser fallback recognizer');
          state.useSpeechFallback = true;
          const started = await postJSON('/conversation/mic/start', { mode: 'manual' }).catch(() => ({}));
          state.sessionID = String(started?.session_id || '');
          state.traceID = String(started?.trace_id || started?.session_id || '');
          state.sessionMode = String(started?.mode || 'manual');
          state.speechRecognition.start();
        });
      }
    } catch (error) {
      const message = String(error && error.message ? error.message : error || 'unknown');
      if (message.includes('native capture unavailable') || message.includes('conversation start failed')) {
        setStatus('当前环境无法启动真实语音输入，且浏览器 fallback 不可用。请检查麦克风权限、后端状态和原生音频链路。', 'error');
      } else {
        setStatus(`语音输入启动失败：${message}`, 'error');
      }
    }
  });

  function focusByNormalized(x, y, instant = false) {
    if (!state.model) return;
    const controller = state.model?.internalModel?.focusController;
    if (controller && typeof controller.focus === 'function') {
      controller.focus(clamp(x, -1, 1), clamp(y, -1, 1), instant);
    }
  }

  function installGazeBridge() {
    const internal = state.model?.internalModel;
    if (!internal || internal.__normaGazePatched) return;

    const originalUpdateFocus = typeof internal.updateFocus === 'function' ? internal.updateFocus.bind(internal) : null;
    internal.updateFocus = function normaUpdateFocus() {
      if (originalUpdateFocus) originalUpdateFocus();
      const core = internal.coreModel;
      if (!core || typeof core.addParameterValueById !== 'function') return;
      const gx = clamp(state.focusX, -1, 1);
      const gy = clamp(-state.focusY, -1, 1);
      core.addParameterValueById('ParamEyeBallX', gx * 1.05);
      core.addParameterValueById('ParamEyeBallY', gy * 1.05);
      core.addParameterValueById('ParamAngleX', gx * 24);
      core.addParameterValueById('ParamAngleY', gy * 18);
      core.addParameterValueById('ParamBodyAngleX', gx * 8);
    };

    internal.__normaGazePatched = true;
  }

  function connectEvents() {
    closeEvents();

    state.eventSources.push(createRuntimeEventSource('live2d.motion', async (payload) => {
      await playMotionWithPolicy(payload || {});
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.emotion', (payload) => {
      const intensity = Number(payload?.intensity);
      if (Number.isFinite(intensity)) state.motionIntensity = clamp(intensity, 0.2, 1);
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.expression', (payload) => {
      if (!payload || typeof payload !== 'object') return;
      playNativeExpression(payload).then((usedNative) => {
        if (usedNative) return;
      const name = String(payload.name || '').toLowerCase();
      const intensity = Number.isFinite(Number(payload.intensity)) ? Number(payload.intensity) : 0.5;
      if (name === 'smile') {
        state.motionState.paramTargets.ParamMouthForm = 0.4 + intensity * 0.35;
        state.motionState.paramTargets.ParamCheek = 0.15 + intensity * 0.35;
      } else if (name === 'alert') {
        state.motionState.paramTargets.ParamBrowAngleY = 0.2 + intensity * 0.25;
        state.motionState.paramTargets.ParamMouthForm = -0.08 - intensity * 0.2;
      } else if (name === 'focus') {
        state.motionState.paramTargets.ParamBrowAngleY = 0.08 + intensity * 0.18;
        state.motionState.paramTargets.ParamEyeLOpen = 1;
        state.motionState.paramTargets.ParamEyeROpen = 1;
      } else if (name === 'soft') {
        state.motionState.paramTargets.ParamBrowAngleY = -0.2;
        state.motionState.paramTargets.ParamEyeLOpen = 0.88;
        state.motionState.paramTargets.ParamEyeROpen = 0.88;
      }
      scheduleExpressionReset(Number.isFinite(Number(payload.hold_ms)) ? Number(payload.hold_ms) : 1200);
      }).catch(() => undefined);
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.parameters', (payload) => {
      const params = payload?.params;
      if (!params || typeof params !== 'object') return;
      Object.entries(params).forEach(([key, value]) => {
        const numeric = Number(value);
        if (Number.isFinite(numeric)) state.motionState.paramTargets[key] = numeric;
      });
      if (Number.isFinite(Number(payload?.blend_ms))) {
        state.motionState.paramBlend = clamp(Number(payload.blend_ms) / 1000, 0.08, 0.35);
      }
      scheduleExpressionReset(Number.isFinite(Number(payload?.hold_ms)) ? Number(payload.hold_ms) : 1200);
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.mouth', (payload) => {
      if (typeof payload === 'number') {
        state.mouthTarget = clamp(payload, 0, 1.2);
        return;
      }
      if (typeof payload?.level === 'number') {
        state.mouthTarget = clamp(payload.level, 0, 1.2);
        return;
      }
      state.mouthTarget = String(payload || '').toLowerCase() === 'speaking' ? 0.7 : 0;
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.mouth.viseme', (payload) => {
      const intensity = Number(payload?.intensity);
      if (Number.isFinite(intensity)) state.mouthTarget = clamp(intensity, 0, 1.2);
      const phoneme = String(payload?.phoneme || 'mid').toLowerCase();
      state.mouthForm = phoneme === 'a' ? 1 : phoneme === 'i' ? -0.45 : phoneme === 'u' ? -0.2 : phoneme === 'o' ? 0.35 : 0.08;
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.eye', (payload) => {
      const x = Number(payload?.x);
      const y = Number(payload?.y);
      if (Number.isFinite(x)) state.targetX = clamp(x, -1, 1);
      if (Number.isFinite(y)) state.targetY = clamp(y, -1, 1);
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.pose', (payload) => {
      if (!payload || typeof payload !== 'object') return;
      const parts = Array.isArray(payload.parts) ? payload.parts : [];
      if (!parts.length) return;
      const core = state.model?.internalModel?.coreModel;
      if (!core || typeof core.setPartOpacityById !== 'function') return;

      const active = String(payload.active || parts[0]);
      parts.forEach((partId) => {
        core.setPartOpacityById(partId, partId === active ? 1 : 0);
      });
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.reload', async (payload) => {
      const preferredURL = normalizeModelURL(payload?.model_url || payload?.url || '');
      await reload(preferredURL);
    }));

    state.eventSources.push(createRuntimeEventSource('tts.chunk.ready', (payload, evt) => {
      if (!payload || typeof payload !== 'object') return;
      if (Number.isFinite(Number(payload.sample_rate))) state.audioFormat.sampleRate = Number(payload.sample_rate);
      if (Number.isFinite(Number(payload.channels))) state.audioFormat.channels = Number(payload.channels);
      if (typeof payload.encoding === 'string' && payload.encoding) state.audioFormat.encoding = payload.encoding;
      if (typeof payload.audio_base64 !== 'string' || !payload.audio_base64) return;
      const evtSessionID = String(evt?.session_id || state.sessionID || '');
      const evtTraceID = String(evt?.trace_id || state.traceID || evtSessionID || '');
      if (window.__NORMA_NATIVE_AUDIO__) return;
      markPlaybackStart(evtSessionID, evtTraceID);
      state.playbackState.pending += 1;
      state.playbackQueue = state.playbackQueue
        .then(() => playPCMChunk(payload.audio_base64))
        .then((samples) => {
          driveMouthFromSamples(samples, evtSessionID, evtTraceID);
          return publish('audio.play.progress', {
            source: 'web-audio',
            bytes: Number(payload.bytes) || 0,
            level: computeChunkLevel(samples)
          }, { sessionID: evtSessionID, traceID: evtTraceID }).catch(() => undefined);
        })
        .finally(() => {
          state.playbackState.pending = Math.max(0, state.playbackState.pending - 1);
          finishPlaybackIfIdle(state.playbackState.interrupted ? 'interrupt' : 'done');
        })
        .catch((error) => console.error(error));
    }));

    state.eventSources.push(createRuntimeEventSource('audio.play.start', (_payload, evt) => {
      const evtSessionID = String(evt?.session_id || state.sessionID || '');
      const evtTraceID = String(evt?.trace_id || state.traceID || evtSessionID || '');
      publish('live2d.mouth', 'speaking', { sessionID: evtSessionID, traceID: evtTraceID }).catch(() => undefined);
      if ((window.__NORMA_KEEP_CAPTURE_ALIVE__ || state.runtimeHints.keepMicCaptureAlive) && !state.useSpeechFallback) {
        startMediaCapture().catch(() => undefined);
      }
    }));

    state.eventSources.push(createRuntimeEventSource('assistant.runtime.hints', (payload) => {
      if (!payload || typeof payload !== 'object') return;
      state.runtimeHints.keepMicCaptureAlive = payload.keep_mic_capture_alive === true;
      state.runtimeHints.needRealAECNS = payload.need_real_aec_ns === true;
    }));

    state.eventSources.push(createRuntimeEventSource('audio.play.end', (_payload, evt) => {
      const evtSessionID = String(evt?.session_id || state.sessionID || '');
      const evtTraceID = String(evt?.trace_id || state.traceID || evtSessionID || '');
      state.mouthTarget = 0;
      publish('live2d.mouth', { level: 0 }, { sessionID: evtSessionID, traceID: evtTraceID }).catch(() => undefined);
    }));

    state.eventSources.push(createRuntimeEventSource('audio.play.interrupt', (_payload, evt) => {
      state.playbackState.pending = 0;
      state.playbackState.interrupted = true;
      const evtSessionID = String(evt?.session_id || state.sessionID || '');
      const evtTraceID = String(evt?.trace_id || state.traceID || evtSessionID || '');
      state.mouthTarget = 0;
      publish('live2d.mouth', { level: 0 }, { sessionID: evtSessionID, traceID: evtTraceID }).catch(() => undefined);
      finishPlaybackIfIdle('interrupt');
    }));

    state.eventSources.push(createRuntimeEventSource('tts.stream.done', () => {
      finishPlaybackIfIdle(state.playbackState.interrupted ? 'interrupt' : 'done');
    }));
  }

  async function loadModel(preferredModelURL = '') {
    setStatus('正在检查 API 服务...', 'info');
    const apiReady = await waitForApiReady(4000);
    if (!apiReady) throw new Error('api server is not ready');

    if (stage) {
      stage.style.opacity = '0';
      stage.style.visibility = 'hidden';
    }

    setStatus('API 已就绪，正在等待舞台尺寸稳定...', 'info');
    const stageReady = await waitForStageReady();

    destroy();
    connectEvents();
    if (!stageReady) {
      setStatus('API 已就绪，舞台未完全稳定，继续快速加载模型...', 'info');
    }
    setStatus('API 已就绪，正在解析模型目录...', 'info');
    const modelMeta = await resolveModelMeta();
    const modelURL = normalizeModelURL(preferredModelURL) || modelMeta.modelURL;
    const coreURL = siblingCoreURL(modelURL);

    setStatus(`API 已就绪，开始加载模型（${modelMeta.modelName || '默认'}）...`, 'info');
    await ensureCore(coreURL);
    if (!state.Live2DModel) {
      const mod = await import('pixi-live2d-display/cubism4');
      state.Live2DModel = mod.Live2DModel;
      if (typeof state.Live2DModel?.registerTicker === 'function' && PIXI.Ticker) {
        state.Live2DModel.registerTicker(PIXI.Ticker);
      }
    }

    state.app = new PIXI.Application({
      resizeTo: stage,
      antialias: true,
      autoDensity: true,
      resolution: window.devicePixelRatio || 1,
      backgroundAlpha: 0
    });
    stage.appendChild(state.app.view);

    state.model = await state.Live2DModel.from(modelURL, { autoInteract: false });
    state.app.stage.addChild(state.model);
    installGazeBridge();
    buildLive2DCatalog(modelMeta.modelMapping || { motions: {}, expressions: {} });
    console.info('[live2d] catalog', debugCatalogSummary());

    state.model.anchor.set(0.5, 1);
    const layout = () => {
      if (!state.model || !stage) return;
      if (stage.clientWidth < 80 || stage.clientHeight < 120) return;
      state.model.x = stage.clientWidth / 2;
      state.model.y = stage.clientHeight * 0.96;
      state.baseX = state.model.x;
      state.baseY = state.model.y;
      const width = Math.max(1, state.model.width);
      const height = Math.max(1, state.model.height);
      const scale = Math.min((stage.clientWidth * 0.92) / width, (stage.clientHeight * 0.96) / height);
      state.model.scale.set(scale > 0 ? scale : 0.32);
    };
    state.layoutHandler = layout;
    layout();
    window.addEventListener('resize', layout);
    if (typeof ResizeObserver !== 'undefined') {
      state.resizeObserver = new ResizeObserver(() => layout());
      state.resizeObserver.observe(stage);
    }
    window.setTimeout(layout, 120);
    window.setTimeout(layout, 420);
    window.requestAnimationFrame(() => window.requestAnimationFrame(layout));

    await state.model.motion('Idle', 0, 1).catch(() => undefined);
    if (stage) {
      stage.style.visibility = 'visible';
      stage.style.opacity = '1';
    }
    setStatus('Live2D 模型已加载', 'ready', true);

    state.app.ticker.add(() => {
      const t = performance.now() / 1000;
      if (state.model) {
        state.model.x = state.baseX + Math.sin(t * 0.9) * 1.5;
        state.model.y = state.baseY + Math.sin(t * 1.2) * 3.5;
        state.model.rotation = Math.sin(t * 0.7) * 0.01;
      }
      if (!state.model?.internalModel?.coreModel) return;
      const core = state.model.internalModel.coreModel;
      state.focusX += (state.targetX - state.focusX) * 0.12;
      state.focusY += (state.targetY - state.focusY) * 0.12;
      focusByNormalized(state.focusX, state.focusY, true);
      state.mouthCurrent += (state.mouthTarget - state.mouthCurrent) * 0.28;
      writeParam(core, 'ParamMouthOpenY', state.mouthCurrent);
      writeParam(core, 'ParamMouthForm', state.mouthForm);
      Object.entries(state.motionState.paramTargets).forEach(([key, target]) => {
        const current = Number.isFinite(state.motionState.paramValues[key]) ? state.motionState.paramValues[key] : 0;
        const next = current + (target - current) * state.motionState.paramBlend;
        state.motionState.paramValues[key] = next;
        writeParam(core, key, next);
      });
      writeParam(core, 'ParamBreath', 0.5 + Math.sin(t * 1.8) * 0.5);
      writeParam(core, 'ParamBodyAngleZ', Math.sin(t * 0.9) * 2.2 * state.motionIntensity);
      writeParam(core, 'ParamAngleZ', Math.sin(t * 1.1) * 2.0 * state.motionIntensity);
    });
  }

  async function reload(preferredModelURL = '') {
    for (let attempt = 1; attempt <= 3; attempt += 1) {
      try {
        await loadModel(preferredModelURL);
        return;
      } catch (err) {
        console.error(`[live2d] init failed (${attempt}/3)`, err);
        setStatus(`模型加载失败，正在重试 ${attempt}/3`, 'error');
        await sleep(Math.min(400 * attempt, 1200));
      }
    }
    setStatus('Live2D 模型加载失败，请检查模型文件和 API 服务', 'error');
  }

  window.addEventListener('beforeunload', destroy);
  reload().catch((err) => {
    console.error(err);
    setStatus('Live2D 初始化失败', 'error');
  });
}

async function boot() {
  apiBase = await detectAPIBase();
  if (mode === 'debug') {
    window.location.replace(`${apiBase}/static/debug/index.html`);
    return;
  }
  if (mode === 'settings') {
    renderSettingsPage();
    return;
  }
  if (mode === 'onboarding') {
    renderOnboardingPage();
    return;
  }
  await initCompanion();
}

boot().catch((err) => {
  console.error(err);
});
