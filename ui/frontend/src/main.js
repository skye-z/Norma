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
      <div id="debug-toast" class="debug-toast hidden"></div>
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
    const text = String(document.getElementById('txt')?.value || '').trim();
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
  const toast = document.getElementById('debug-toast');
  const state = {
    app: null,
    model: null,
    mouthTarget: 0,
    mouthCurrent: 0,
    targetX: 0,
    targetY: 0,
    eyeX: 0,
    eyeY: 0,
    eventSources: [],
    toastQueue: [],
    toastRunning: false,
    baseX: 0,
    baseY: 0
  };

  function enqueueToast(text) {
    if (!toast || !text) return;
    state.toastQueue.push(String(text));
    if (!state.toastRunning) {
      drainToasts();
    }
  }

  async function drainToasts() {
    if (!toast || state.toastRunning) return;
    state.toastRunning = true;
    while (state.toastQueue.length > 0) {
      const next = state.toastQueue.shift();
      toast.textContent = next;
      toast.classList.remove('hidden');
      toast.classList.add('visible');
      await sleep(3000);
      toast.classList.remove('visible');
      toast.classList.add('hidden');
      toast.textContent = '';
      await sleep(180);
    }
    state.toastRunning = false;
  }

  function normalizeMotion(payload) {
    const aliases = { thinking: 'Tap', tapbody: 'Tap@Body', flickbody: 'Flick@Body', idle: 'Idle' };
    if (typeof payload === 'string') {
      const key = payload.replace(/[^a-zA-Z]/g, '').toLowerCase();
      return aliases[key] || payload;
    }
    const raw = String(payload?.group || payload?.motion || payload?.name || 'Idle');
    const key = raw.replace(/[^a-zA-Z]/g, '').toLowerCase();
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
      enqueueToast(`cmd motion: ${group}#${index}`);
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

    const mouthES = createEventSource('live2d.mouth', (payload) => {
      if (typeof payload === 'number') {
        state.mouthTarget = payload;
      } else if (typeof payload?.level === 'number') {
        state.mouthTarget = payload.level;
      } else {
        state.mouthTarget = String(payload || '').toLowerCase() === 'speaking' ? 0.7 : 0;
      }
      enqueueToast(`cmd mouth: ${state.mouthTarget.toFixed(2)}`);
    });

    const eyeES = createEventSource('live2d.eye', (payload) => {
      if (Number.isFinite(Number(payload?.x))) state.targetX = Number(payload.x);
      if (Number.isFinite(Number(payload?.y))) state.targetY = Number(payload.y);
      enqueueToast(`cmd eye: (${state.targetX.toFixed(2)}, ${state.targetY.toFixed(2)})`);
    });

    const reloadES = createEventSource('live2d.reload', async () => {
      enqueueToast('runtime reload received');
      await reloadCompanion();
    });

    const noticeES = createEventSource('debug.notice', (payload) => {
      enqueueToast(typeof payload?.text === 'string' ? payload.text : String(payload || 'debug.notice'));
    });

    state.eventSources.push(motionES, mouthES, eyeES, reloadES, noticeES);
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
    state.eyeX = 0;
    state.eyeY = 0;
    if (stage) stage.innerHTML = '';
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

  async function initCompanion() {
    enqueueToast('init: waiting api server');
    const apiReady = await waitForApiReady(8000);
    if (!apiReady) {
      enqueueToast('init failed: api server not ready');
      throw new Error('api server is not ready');
    }
    enqueueToast('init: api ready');

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

    state.model.anchor.set(0.5, 1);
    state.model.x = stage.clientWidth / 2;
    state.model.y = stage.clientHeight * 0.985;
    state.baseX = state.model.x;
    state.baseY = state.model.y;
    state.model.scale.set(Math.min(stage.clientWidth * 1.08 / state.model.width, stage.clientHeight * 1.22 / state.model.height));
    await state.model.motion('Idle', 0, 1).catch(() => undefined);
    enqueueToast('state -> idle');

    state.app.ticker.add(() => {
      const t = performance.now() / 1000;
      if (state.model) {
        state.model.x = state.baseX + Math.sin(t * 0.9) * 1.5;
        state.model.y = state.baseY + Math.sin(t * 1.2) * 3.5;
        state.model.rotation = Math.sin(t * 0.7) * 0.01;
      }
      if (!state.model?.internalModel?.coreModel) return;
      const core = state.model.internalModel.coreModel;

      state.mouthCurrent += (state.mouthTarget - state.mouthCurrent) * 0.28;
      core.setParameterValueById('ParamMouthOpenY', state.mouthCurrent);

      state.eyeX += (state.targetX - state.eyeX) * 0.18;
      state.eyeY += (state.targetY - state.eyeY) * 0.18;
      core.setParameterValueById('ParamEyeBallX', state.eyeX);
      core.setParameterValueById('ParamEyeBallY', state.eyeY);
      core.setParameterValueById('ParamAngleX', state.eyeX * 18);
      core.setParameterValueById('ParamAngleY', state.eyeY * 12);
      core.setParameterValueById('ParamBodyAngleX', state.eyeX * 8);
      core.setParameterValueById('ParamBreath', 0.5 + Math.sin(t * 1.8) * 0.5);
      core.setParameterValueById('ParamBodyAngleZ', Math.sin(t * 0.9) * 2.2);
      core.setParameterValueById('ParamAngleZ', Math.sin(t * 1.1) * 2.0);
    });
  }

  async function reloadCompanion() {
    for (let i = 1; i <= 6; i += 1) {
      try {
        await initCompanion();
        return;
      } catch (err) {
        enqueueToast(`init retry ${i}/6`);
        console.error(`[live2d] init failed (attempt ${i}/6)`, err);
        await sleep(Math.min(1200 * i, 5000));
      }
    }
    enqueueToast('init aborted after retries');
  }

  function setEyeTargetByClientPoint(clientX, clientY) {
    const r = stage.getBoundingClientRect();
    state.targetX = Math.max(-1, Math.min(1, ((clientX - r.left) / Math.max(1, r.width) - 0.5) * 2));
    state.targetY = Math.max(-1, Math.min(1, ((clientY - r.top) / Math.max(1, r.height) - 0.5) * 2));
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
  });

  window.addEventListener('beforeunload', () => {
    destroyRenderer();
  });

  reloadCompanion().catch((err) => {
    console.error(err);
  });
}
