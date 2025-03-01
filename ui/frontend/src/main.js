import './style.css';
import * as PIXI from 'pixi.js';

const apiBase = 'http://127.0.0.1:8090';
const modelURL = `${apiBase}/static/model/runtime/hiyori_pro_t11.model3.json`;
const coreURL = `${apiBase}/static/model/runtime/live2dcubismcore.min.js`;

const params = new URLSearchParams(window.location.search);
const isDebug = params.get('debug') === '1';
const isSettings = params.get('settings') === '1';

if (isSettings) {
  document.querySelector('#app').innerHTML = `<main class="settings-page"><section class="settings-panel"><h1>Norma Settings</h1></section></main>`;
} else if (isDebug) {
  document.querySelector('#app').innerHTML = `
    <main class="debug-page">
      <section class="debug-panel">
        <h1>Norma Debug Panel</h1>
        <div class="speech-row">
          <input id="txt" class="txt" value="Hello, I am Norma." />
          <button id="speak" class="btn">Speak</button>
        </div>
        <div class="control-grid">
          <button class="btn m" data-m="Idle">Idle</button>
          <button class="btn m" data-m="Tap">Tap</button>
          <button class="btn m" data-m="Flick">Flick</button>
        </div>
        <div class="range-row"><label>Mouth</label><input id="mouth" type="range" min="0" max="1.2" step="0.01" value="0" /></div>
        <div class="range-row"><label>EyeX</label><input id="ex" type="range" min="-1" max="1" step="0.01" value="0" /></div>
        <div class="range-row"><label>EyeY</label><input id="ey" type="range" min="-1" max="1" step="0.01" value="0" /></div>
        <pre id="log"></pre>
      </section>
    </main>
  `;
} else {
  document.querySelector('#app').innerHTML = `
    <main class="companion-shell">
      <section id="debug-log-panel" class="debug-log-panel">
        <ul id="debug-log-list" class="debug-log-list"></ul>
      </section>
      <div id="stage" class="stage companion-stage wails-drag"></div>
    </main>
  `;
}

const logEl = document.getElementById('log');
const log = (msg) => {
  if (!logEl) return;
  const ts = new Date().toLocaleTimeString();
  logEl.textContent = `[${ts}] ${msg}\n` + logEl.textContent;
};

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitForApiReady(timeoutMs = 10000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const resp = await fetch(`${apiBase}/healthz`, { cache: 'no-store' });
      if (resp.ok) {
        return true;
      }
    } catch (_) {}
    await sleep(350);
  }
  return false;
}

async function publish(name, payload) {
  try {
    await fetch(`${apiBase}/runtime/publish`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, payload })
    });
  } catch (err) {
    log(`publish failed: ${String(err)}`);
  }
}

if (isDebug) {
  document.querySelectorAll('.m').forEach((btn) => {
    btn.addEventListener('click', () => {
      const group = btn.dataset.m;
      publish('live2d.motion', { group, index: 0 });
      publish('debug.notice', { text: `motion -> ${group}` });
    });
  });

  document.getElementById('mouth')?.addEventListener('input', (e) => {
    const level = Number(e.target.value);
    publish('live2d.mouth', { level });
    publish('debug.notice', { text: `mouth -> ${level.toFixed(2)}` });
  });

  const emitEye = () => {
    const x = Number(document.getElementById('ex')?.value || 0);
    const y = Number(document.getElementById('ey')?.value || 0);
    publish('live2d.eye', { x, y });
    publish('debug.notice', { text: `eye -> (${x.toFixed(2)}, ${y.toFixed(2)})` });
  };

  document.getElementById('ex')?.addEventListener('input', emitEye);
  document.getElementById('ey')?.addEventListener('input', emitEye);

  document.getElementById('speak')?.addEventListener('click', () => {
    const text = String(document.getElementById('txt')?.value || ').trim();
    if (!text) return;
    publish('debug.notice', { text: `speak -> ${text.slice(0, 24)}` });

    const utter = new SpeechSynthesisUtterance(text);
    utter.lang = 'en-US';
    const timer = setInterval(() => publish('live2d.mouth', { level: 0.2 + Math.random() * 0.8 }), 90);
    utter.onend = () => {
      clearInterval(timer);
      publish('live2d.mouth', { level: 0 });
    };
    utter.onerror = utter.onend;
    speechSynthesis.cancel();
    speechSynthesis.speak(utter);
  });
}
if (!isDebug && !isSettings) {
  const stage = document.getElementById('stage');
  const logList = document.getElementById('debug-log-list');
  const state = {
    app: null,
    model: null,
    mouthTarget: 0,
    mouthCurrent: 0,
    mouthForm: 0,
    targetX: 0,
    targetY: 0,
    eyeX: 0,
    eyeY: 0,
    emotion: 'neutral',
    motionIntensity: 0.45,
    eventSources: [],
    logs: [],
    baseX: 0,
    baseY: 0
  };

  function appendLog(text) {
    if (!logList || !text) return;
    const line = String(text);
    const nearBottom = logList.scrollTop + logList.clientHeight >= logList.scrollHeight - 8;
    state.logs.push(line);
    if (state.logs.length > 30) {
      state.logs.splice(0, state.logs.length - 30);
    }
    while (logList.firstChild) {
      logList.removeChild(logList.firstChild);
    }
    state.logs.forEach((item) => {
      const li = document.createElement('li');
      li.textContent = item;
      logList.appendChild(li);
    });
    if (nearBottom) {
      logList.scrollTop = logList.scrollHeight;
    }
  }

  function normalizeMotion(payload) {
    const aliases = { thinking: 'Tap', tapbody: 'Tap@Body', flickbody: 'Flick@Body', idle: 'Idle' };
    if (typeof payload === 'string') {
      const key = payload.replace(/[^a-zA-Z]/g, ').toLowerCase();
      return aliases[key] || payload;
    }
    const raw = String(payload?.group || payload?.motion || payload?.name || 'Idle');
    const key = raw.replace(/[^a-zA-Z]/g, ').toLowerCase();
    return aliases[key] || raw;
  }

  function closeEvents() {
    state.eventSources.forEach((es) => {
      try {
        es.close();
      } catch (_) {}
    });
    state.eventSources = [];
  }

  function createEventSource(name, onPayload) {
    const es = new EventSource(`${apiBase}/runtime/events?name=${encodeURIComponent(name)}`);
    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        onPayload(data.payload);
      } catch (_) {}
    };
    es.onerror = () => {
      console.warn(`[live2d] event source reconnecting: ${name}`);
    };
    return es;
  }

  function connectEvents() {
    closeEvents();

    const motionES = createEventSource('live2d.motion', async (payload) => {
      if (!state.model) return;
      const group = normalizeMotion(payload);
      const index = Number.isInteger(payload?.index) ? payload.index : 0;
      appendLog(`cmd motion: ${group}#${index}`);
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
        });

    const emotionES = createEventSource('live2d.emotion', (payload) => {
      if (typeof payload?.emotion === 'string') {
        state.emotion = payload.emotion;
      }
      if (Number.isFinite(Number(payload?.intensity))) {
        state.motionIntensity = Number(payload.intensity);
      }
      appendLog('cmd emotion: ' + state.emotion + ' / ' + state.motionIntensity.toFixed(2));
    });

    const mouthES = createEventSource('live2d.mouth', (payload) => {
      if (typeof payload === 'number') {
        state.mouthTarget = payload;
      } else if (typeof payload?.level === 'number') {
        state.mouthTarget = payload.level;
      } else {
        state.mouthTarget = String(payload || '').toLowerCase() === 'speaking' ? 0.7 : 0;
      }
      appendLog('cmd mouth: ' + state.mouthTarget.toFixed(2));
    });

    const visemeES = createEventSource('live2d.mouth.viseme', (payload) => {
      if (Number.isFinite(Number(payload?.intensity))) {
        state.mouthTarget = Number(payload.intensity);
      }
      const phoneme = String(payload?.phoneme || 'mid');
      state.mouthForm = phoneme === 'a' ? 1 : phoneme === 'i' ? -0.5 : phoneme === 'u' ? -0.2 : phoneme === 'o' ? 0.4 : 0.1;
    });

    const eyeES = createEventSource('live2d.eye', (payload) => {
      if (Number.isFinite(Number(payload?.x))) state.targetX = Number(payload.x);
      if (Number.isFinite(Number(payload?.y))) state.targetY = Number(payload.y);
      applyFocusNormalized(state.targetX, state.targetY);
      appendLog(`cmd eye: (${state.targetX.toFixed(2)}, ${state.targetY.toFixed(2)})`);
    });

    const reloadES = createEventSource('live2d.reload', async () => {
      appendLog('runtime reload received');
      await reloadCompanion();
    });

    const noticeES = createEventSource('debug.notice', (payload) => {
      appendLog(typeof payload?.text === 'string' ? payload.text : String(payload || 'debug.notice'));
    });

    state.eventSources.push(motionES, emotionES, mouthES, visemeES, eyeES, reloadES, noticeES);
  }

  function destroyRenderer() {
    closeEvents();
    if (state.app) {
      try {
        state.app.destroy(true, { children: true, texture: true, baseTexture: true });
      } catch (_) {}
      state.app = null;
    }
    state.model = null;
    state.mouthTarget = 0;
    state.mouthCurrent = 0;
    state.mouthForm = 0;
    state.eyeX = 0;
    state.eyeY = 0;
    state.emotion = 'neutral';
    state.motionIntensity = 0.45;
    if (stage) stage.innerHTML = ';
  }

  async function ensureCore() {
    if (window.Live2DCubismCore) return;
    await new Promise((resolve, reject) => {
      const s = document.createElement('script');
      s.src = coreURL;
      s.async = true;
      s.onload = resolve;
      s.onerror = () => reject(new Error('live2d core load failed'));
      document.head.appendChild(s);
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
  function focusByNormalized(x, y, instant = false) {
    if (!state.model) return;
    const nx = Math.max(-1, Math.min(1, x));
    const ny = Math.max(-1, Math.min(1, y));
    const controller = state.model?.internalModel?.focusController;
    if (controller && typeof controller.focus === 'function') {
      controller.focus(nx, ny, instant);
    }
  }

  function installGazeBridge() {
    const internal = state.model?.internalModel;
    if (!internal || internal.__normaGazePatched) return;

    const originalUpdateFocus = typeof internal.updateFocus === 'function' ? internal.updateFocus.bind(internal) : null;

    internal.updateFocus = function normaUpdateFocus() {
      if (originalUpdateFocus) {
        originalUpdateFocus();
      }
      const core = internal.coreModel;
      if (!core || typeof core.addParameterValueById !== 'function') return;

      const gx = Math.max(-1, Math.min(1, state.targetX));
      const gy = Math.max(-1, Math.min(1, state.targetY));
      core.addParameterValueById('ParamEyeBallX', gx * 1.1);
      core.addParameterValueById('ParamEyeBallY', gy * 1.1);
      core.addParameterValueById('ParamAngleX', gx * 26);
      core.addParameterValueById('ParamAngleY', gy * 20);
      core.addParameterValueById('ParamBodyAngleX', gx * 10);
    };

    internal.__normaGazePatched = true;
    appendLog('gaze bridge installed');
  }
  async function initCompanion() {
    appendLog('init: waiting api server');
    const apiReady = await waitForApiReady(8000);
    if (!apiReady) {
      appendLog('init failed: api server not ready');
      throw new Error('api server is not ready');
    }
    appendLog('init: api ready');

    destroyRenderer();
    connectEvents();

    await ensureCore();
    const mod = await import('pixi-live2d-display/cubism4');

    state.app = new PIXI.Application({
      resizeTo: stage,
      antialias: true,
      autoDensity: true,
      resolution: window.devicePixelRatio || 1,
      backgroundAlpha: 0
    });
    stage.appendChild(state.app.view);

    state.model = await mod.Live2DModel.from(modelURL, { autoInteract: false });
    state.app.stage.addChild(state.model);
    installGazeBridge();

    state.model.anchor.set(0.5, 1);
    state.model.x = stage.clientWidth / 2;
    state.model.y = stage.clientHeight * 0.985;
    state.baseX = state.model.x;
    state.baseY = state.model.y;
    state.model.scale.set(Math.min(stage.clientWidth * 1.08 / state.model.width, stage.clientHeight * 1.22 / state.model.height));
    await state.model.motion('Idle', 0, 1).catch(() => undefined);
    appendLog('state -> idle');

    state.app.ticker.add(() => {
      const t = performance.now() / 1000;
      if (state.model) {
        state.model.x = state.baseX + Math.sin(t * 0.9) * 1.5;
        state.model.y = state.baseY + Math.sin(t * 1.2) * 3.5;
        state.model.rotation = Math.sin(t * 0.7) * 0.01;
      }
      if (!state.model?.internalModel?.coreModel) return;
      const core = state.model.internalModel.coreModel;

      focusByNormalized(state.targetX, state.targetY, true);

      state.mouthCurrent += (state.mouthTarget - state.mouthCurrent) * 0.28;
      writeParam(core, 'ParamMouthOpenY', state.mouthCurrent);
      writeParam(core, 'ParamMouthForm', state.mouthForm);
      writeParam(core, 'ParamBreath', 0.5 + Math.sin(t * 1.8) * 0.5);
      writeParam(core, 'ParamBodyAngleZ', Math.sin(t * 0.9) * 2.2 * state.motionIntensity);
      writeParam(core, 'ParamAngleZ', Math.sin(t * 1.1) * 2.0 * state.motionIntensity);
    });
  }

  async function reloadCompanion() {
    for (let i = 1; i <= 6; i += 1) {
      try {
        await initCompanion();
        return;
      } catch (err) {
        appendLog(`init retry ${i}/6`);
        console.error(`[live2d] init failed (attempt ${i}/6)`, err);
        await sleep(Math.min(1200 * i, 5000));
      }
    }
    appendLog('init aborted after retries');
  }

  function setEyeTargetByClientPoint(clientX, clientY) {
    const r = stage.getBoundingClientRect();
    const x = Math.max(-1, Math.min(1, ((clientX - r.left) / Math.max(1, r.width) - 0.5) * 2));
    const y = Math.max(-1, Math.min(1, ((clientY - r.top) / Math.max(1, r.height) - 0.5) * 2));
    state.targetX = x;
    state.targetY = y;
    applyFocusNormalized(x, y);
  }

  function applyFocusNormalized(x, y, instant = false) {
    focusByNormalized(x, y, instant);
  }


  stage.addEventListener('mousemove', (evt) => {
    setEyeTargetByClientPoint(evt.clientX, evt.clientY);
  });

  window.addEventListener('mousemove', (evt) => {
    setEyeTargetByClientPoint(evt.clientX, evt.clientY);
  });

  stage.addEventListener('mouseleave', () => {
    state.targetX = 0;
    state.targetY = 0;
    applyFocusNormalized(0, 0);
  });

  window.addEventListener('beforeunload', () => {
    destroyRenderer();
  });

  reloadCompanion().catch((err) => {
    console.error(err);
  });
}
