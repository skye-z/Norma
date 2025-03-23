import './style.css';
import * as PIXI from 'pixi.js';

window.PIXI = PIXI;

const apiBase = 'http://127.0.0.1:8090';
const modelURL = `${apiBase}/static/model/runtime/hiyori_pro_t11.model3.json`;
const coreURL = `${apiBase}/static/model/runtime/live2dcubismcore.min.js`;

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

async function publish(name, payload) {
  return postJSON('/runtime/publish', {
    name,
    session_id: `wails-${mode}`,
    trace_id: `wails-${mode}`,
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
  const es = new EventSource(`${apiBase}/runtime/events?name=${encodeURIComponent(name)}`);
  es.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      onPayload(data.payload, data);
    } catch (_) {}
  };
  es.onerror = () => {
    if (typeof onError === 'function') onError(name);
  };
  return es;
}

function navTabs(active) {
  const tabs = [
    ['http://127.0.0.1:8090/static/debug/home.html', '控制台', 'home'],
    ['?settings=1', '设置', 'settings'],
    ['http://127.0.0.1:8090/static/debug/home.html', '控制台首页', 'external']
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

async function ensureCore() {
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
    resizeObserver: null,
    layoutHandler: null,
    speechRecognition: null,
    speechListening: false
  };

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
      publish('audio.vad.start', { source: 'mic-fab' }).catch(() => undefined);
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
        publish('asr.partial', interimText).catch(() => undefined);
      }
      if (finalText) {
        setStatus(`已识别：${finalText}`, 'ready', true);
        publish('asr.final', finalText).catch(() => undefined);
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
    if (state.speechRecognition && state.speechListening) {
      try { state.speechRecognition.stop(); } catch (_) {}
    }
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
    if (!state.speechRecognition) {
      setStatus('当前环境不支持语音输入，请在支持语音识别的环境中使用。', 'error');
      return;
    }
    try {
      if (state.speechListening) {
        state.speechRecognition.stop();
      } else {
        state.speechRecognition.start();
      }
    } catch (error) {
      setStatus(`语音输入启动失败：${error}`, 'error');
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
      if (!state.model) return;
      const group = normalizeMotionName(payload);
      const index = Number.isInteger(payload?.index) ? payload.index : 0;
      try {
        await state.model.motion(group, index, 3);
      } catch (_) {
        for (const fallback of ['Tap', 'Flick', 'Idle']) {
          try {
            await state.model.motion(fallback, 0, 3);
            break;
          } catch (_) {}
        }
      }
    }));

    state.eventSources.push(createRuntimeEventSource('live2d.emotion', (payload) => {
      const intensity = Number(payload?.intensity);
      if (Number.isFinite(intensity)) state.motionIntensity = clamp(intensity, 0.2, 1);
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

    state.eventSources.push(createRuntimeEventSource('live2d.reload', async () => {
      await reload();
    }));
  }

  async function loadModel() {
    setStatus('正在检查 API 服务...', 'info');
    const apiReady = await waitForApiReady(4000);
    if (!apiReady) throw new Error('api server is not ready');

    setStatus('API 已就绪，正在等待舞台尺寸稳定...', 'info');
    const stageReady = await waitForStageReady();

    destroy();
    connectEvents();
    if (!stageReady) {
      setStatus('API 已就绪，舞台未完全稳定，继续快速加载模型...', 'info');
    }
    setStatus('API 已就绪，开始加载模型...', 'info');
    await ensureCore();
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
      writeParam(core, 'ParamBreath', 0.5 + Math.sin(t * 1.8) * 0.5);
      writeParam(core, 'ParamBodyAngleZ', Math.sin(t * 0.9) * 2.2 * state.motionIntensity);
      writeParam(core, 'ParamAngleZ', Math.sin(t * 1.1) * 2.0 * state.motionIntensity);
    });
  }

  async function reload() {
    for (let attempt = 1; attempt <= 3; attempt += 1) {
      try {
        await loadModel();
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
  if (mode === 'debug') {
    window.location.replace('http://127.0.0.1:8090/static/debug/home.html');
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
