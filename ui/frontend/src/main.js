import './style.css';
import * as PIXI from 'pixi.js';

window.PIXI = PIXI;

const eventApiBase = 'http://127.0.0.1:8090';
const modelUrl = `${eventApiBase}/static/model/runtime/hiyori_pro_t11.model3.json`;
const cubismCoreCandidates = [
  `${eventApiBase}/static/model/runtime/live2dcubismcore.min.js`,
  'https://cubism.live2d.com/sdk-web/cubismcore/live2dcubismcore.min.js'
];

document.querySelector('#app').innerHTML = `
  <main class="page">
    <section class="panel stage-panel">
      <h1>Norma Live2D Preview</h1>
      <p class="hint">Model: <code>/model/runtime/hiyori_pro_t11.model3.json</code></p>
      <div id="stage" class="stage"></div>
      <div class="row"><span>Motion:</span> <strong id="motion">-</strong></div>
      <div class="row"><span>Mouth:</span> <strong id="mouth">-</strong></div>
      <div class="row"><span>Status:</span> <strong id="status">initializing...</strong></div>
      <div class="actions">
        <button id="refresh" class="btn">Reconnect SSE</button>
        <button id="passthrough" class="btn secondary">Enable Click-Through</button>
      </div>
    </section>
    <section class="panel logs">
      <h2>Runtime Events</h2>
      <pre id="log"></pre>
    </section>
  </main>
`;

const motionEl = document.getElementById('motion');
const mouthEl = document.getElementById('mouth');
const statusEl = document.getElementById('status');
const logEl = document.getElementById('log');
const refreshBtn = document.getElementById('refresh');
const passthroughBtn = document.getElementById('passthrough');
const stageEl = document.getElementById('stage');

const MOTION_ALIAS = {
  idle: 'Idle',
  tap: 'Tap',
  flick: 'Flick',
  flickdown: 'FlickDown',
  flickup: 'FlickUp',
  tapbody: 'Tap@Body',
  flickbody: 'Flick@Body'
};

let app;
let model;
let Live2DModel;
let mouthTarget = 0;
let mouthCurrent = 0;
let closers = [];
let mousePassthrough = false;

function appendLog(line) {
  const ts = new Date().toLocaleTimeString();
  logEl.textContent = `[${ts}] ${line}\n` + logEl.textContent;
}

function wailsApp() {
  if (window.go && window.go.main && window.go.main.App) {
    return window.go.main.App;
  }
  return null;
}

function syncPassthroughBtn() {
  passthroughBtn.textContent = mousePassthrough ? 'Disable Click-Through' : 'Enable Click-Through';
  passthroughBtn.classList.toggle('active', mousePassthrough);
}

async function togglePassthrough() {
  const appApi = wailsApp();
  if (!appApi || typeof appApi.SetMousePassthrough !== 'function') {
    appendLog('wails bridge unavailable: click-through toggle only works in desktop app');
    return;
  }

  const next = !mousePassthrough;
  const result = await appApi.SetMousePassthrough(next);
  if (typeof result === 'string' && result.startsWith('ok')) {
    mousePassthrough = next;
    syncPassthroughBtn();
    appendLog(`click-through: ${mousePassthrough ? 'enabled' : 'disabled'}`);
    return;
  }

  appendLog(`click-through failed: ${String(result)}`);
}

async function hydratePassthrough() {
  const appApi = wailsApp();
  if (!appApi || typeof appApi.MousePassthrough !== 'function') {
    return;
  }
  try {
    mousePassthrough = Boolean(await appApi.MousePassthrough());
    syncPassthroughBtn();
  } catch {
    // keep default state in browser mode
  }
}

function loadScript(src) {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = src;
    script.async = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`load script failed: ${src}`));
    document.head.appendChild(script);
  });
}

async function ensureCubismCore() {
  if (window.Live2DCubismCore) {
    return;
  }

  let lastErr;
  for (const src of cubismCoreCandidates) {
    try {
      await loadScript(src);
      if (window.Live2DCubismCore) {
        appendLog(`cubism core loaded: ${src}`);
        return;
      }
    } catch (err) {
      lastErr = err;
    }
  }

  throw lastErr || new Error('cubism core unavailable');
}

async function ensureLive2DModule() {
  if (Live2DModel) {
    return;
  }

  const mod = await import('pixi-live2d-display/cubism4');
  Live2DModel = mod.Live2DModel;
}

function parseMouthLevel(payload) {
  if (typeof payload === 'number') {
    return payload;
  }
  if (payload && typeof payload === 'object') {
    if (typeof payload.level === 'number') {
      return payload.level;
    }
    if (typeof payload.value === 'number') {
      return payload.value;
    }
    if (typeof payload.open === 'number') {
      return payload.open;
    }
  }
  return 0;
}

function normalizeMotion(payload) {
  if (!payload) {
    return { group: 'Idle', index: 0 };
  }

  if (typeof payload === 'string') {
    const key = payload.replace(/[^a-zA-Z]/g, '').toLowerCase();
    return { group: MOTION_ALIAS[key] || payload, index: 0 };
  }

  if (typeof payload === 'object') {
    const rawGroup = String(payload.group || payload.name || payload.motion || 'Idle');
    const key = rawGroup.replace(/[^a-zA-Z]/g, '').toLowerCase();
    const group = MOTION_ALIAS[key] || rawGroup;
    const index = Number.isInteger(payload.index) ? payload.index : 0;
    return { group, index };
  }

  return { group: 'Idle', index: 0 };
}

function connectEvent(name, onData) {
  const es = new EventSource(`${eventApiBase}/runtime/events?name=${encodeURIComponent(name)}`);
  es.onopen = () => {
    statusEl.textContent = 'connected';
    appendLog(`connected: ${name}`);
  };
  es.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      onData(data);
      appendLog(`${name}: ${JSON.stringify(data.payload)}`);
    } catch (err) {
      appendLog(`parse error: ${String(err)}`);
    }
  };
  es.onerror = () => {
    statusEl.textContent = 'disconnected';
    appendLog(`disconnected: ${name}`);
  };
  return () => es.close();
}

function connectAll() {
  closers.forEach((c) => c());
  closers = [];

  closers.push(
    connectEvent('live2d.motion', async (evt) => {
      const m = normalizeMotion(evt.payload);
      motionEl.textContent = `${m.group}[${m.index}]`;
      if (!model) {
        return;
      }
      try {
        await model.motion(m.group, m.index, 3);
      } catch (err) {
        appendLog(`motion failed: ${String(err)}`);
      }
    })
  );

  closers.push(
    connectEvent('live2d.mouth', (evt) => {
      const level = Math.max(0, Math.min(1.2, parseMouthLevel(evt.payload)));
      mouthTarget = level;
      mouthEl.textContent = level.toFixed(2);
    })
  );
}

function layoutModel() {
  if (!app || !model || !stageEl) {
    return;
  }

  const width = stageEl.clientWidth || 360;
  const height = stageEl.clientHeight || 480;
  app.renderer.resize(width, height);

  const sx = width * 0.65 / model.width;
  const sy = height * 0.88 / model.height;
  const scale = Math.min(sx, sy);
  model.scale.set(scale);
  model.x = width / 2;
  model.y = height * 0.96;
  model.anchor.set(0.5, 1);
}

async function initLive2D() {
  app = new PIXI.Application({
    resizeTo: stageEl,
    autoStart: true,
    antialias: true,
    backgroundAlpha: 0
  });
  stageEl.innerHTML = '';
  stageEl.appendChild(app.view);

  try {
    await ensureCubismCore();
    await ensureLive2DModule();

    model = await Live2DModel.from(modelUrl, { autoInteract: false });
    app.stage.addChild(model);
    layoutModel();

    app.ticker.add(() => {
      if (!model || !model.internalModel || !model.internalModel.coreModel) {
        return;
      }
      mouthCurrent += (mouthTarget - mouthCurrent) * 0.28;
      model.internalModel.coreModel.setParameterValueById('ParamMouthOpenY', mouthCurrent);
    });

    await model.motion('Idle', 0, 1);
    appendLog('live2d loaded');
    statusEl.textContent = 'ready';
  } catch (err) {
    statusEl.textContent = 'live2d load failed';
    appendLog(`live2d load failed: ${String(err)}`);
  }
}

window.addEventListener('resize', () => {
  layoutModel();
});

refreshBtn.addEventListener('click', connectAll);
passthroughBtn.addEventListener('click', togglePassthrough);

syncPassthroughBtn();
hydratePassthrough();
initLive2D();
connectAll();
