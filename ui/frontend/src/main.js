
import './style.css';
import * as PIXI from 'pixi.js';

const apiBase = 'http://127.0.0.1:8090';
const modelURL = `${apiBase}/static/model/runtime/hiyori_pro_t11.model3.json`;
const coreURL = `${apiBase}/static/model/runtime/live2dcubismcore.min.js`;

const params = new URLSearchParams(window.location.search);
const isDebug = params.get('debug') === '1';
const isSettings = params.get('settings') === '1';

if (isSettings) {
  document.querySelector('#app').innerHTML = `<main class="settings-page"><section class="settings-panel"><h1>Norma 设置</h1></section></main>`;
} else if (isDebug) {
  document.querySelector('#app').innerHTML = `
    <main class="debug-page">
      <section class="debug-panel">
        <h1>Norma 调试面板</h1>
        <div class="speech-row">
          <input id="txt" class="txt" value="你好，我是诺玛。" />
          <button id="speak" class="btn">语音预览</button>
        </div>
        <div class="control-grid">
          <button class="btn m" data-m="Idle">Idle</button>
          <button class="btn m" data-m="Tap">Tap</button>
          <button class="btn m" data-m="Flick">Flick</button>
        </div>
        <div class="range-row"><label>口型</label><input id="mouth" type="range" min="0" max="1.2" step="0.01" value="0" /></div>
        <div class="range-row"><label>视线X</label><input id="ex" type="range" min="-1" max="1" step="0.01" value="0" /></div>
        <div class="range-row"><label>视线Y</label><input id="ey" type="range" min="-1" max="1" step="0.01" value="0" /></div>
        <pre id="log"></pre>
      </section>
    </main>
  `;
} else {
  document.querySelector('#app').innerHTML = `
    <main class="companion-shell">
      <div id="stage" class="stage companion-stage wails-drag"></div>
      <div id="status" class="status-dot">正在初始化...</div>
    </main>
  `;
}

const logEl = document.getElementById('log');
const log = (msg) => {
  if (!logEl) return;
  const ts = new Date().toLocaleTimeString();
  logEl.textContent = `[${ts}] ${msg}\n` + logEl.textContent;
};

async function publish(name, payload) {
  try {
    await fetch(`${apiBase}/runtime/publish`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, payload })
    });
  } catch (err) {
    log(`发送失败: ${String(err)}`);
  }
}

if (isDebug) {
  document.querySelectorAll('.m').forEach((btn) => {
    btn.addEventListener('click', () => publish('live2d.motion', { group: btn.dataset.m, index: 0 }));
  });

  document.getElementById('mouth')?.addEventListener('input', (e) => {
    publish('live2d.mouth', { level: Number(e.target.value) });
  });

  const emitEye = () => {
    publish('live2d.eye', {
      x: Number(document.getElementById('ex')?.value || 0),
      y: Number(document.getElementById('ey')?.value || 0)
    });
  };
  document.getElementById('ex')?.addEventListener('input', emitEye);
  document.getElementById('ey')?.addEventListener('input', emitEye);

  document.getElementById('speak')?.addEventListener('click', () => {
    const text = String(document.getElementById('txt')?.value || '').trim();
    if (!text) return;

    const utter = new SpeechSynthesisUtterance(text);
    utter.lang = 'zh-CN';
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
  const statusEl = document.getElementById('status');

  const state = {
    app: null,
    model: null,
    mouthTarget: 0,
    mouthCurrent: 0,
    targetX: 0,
    targetY: 0,
    eyeX: 0,
    eyeY: 0,
    eventSources: []
  };

  const setStatus = (text) => {
    if (statusEl) statusEl.textContent = text;
  };

  function closeEvents() {
    state.eventSources.forEach((es) => {
      try { es.close(); } catch (_) {}
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
    return es;
  }

  function connectEvents() {
    closeEvents();

    const motionES = createEventSource('live2d.motion', async (payload) => {
      if (!state.model) return;
      const group = typeof payload === 'string' ? payload : String(payload?.group || 'Idle');
      const index = Number.isInteger(payload?.index) ? payload.index : 0;
      await state.model.motion(group, index, 3).catch(() => undefined);
    });

    const mouthES = createEventSource('live2d.mouth', (payload) => {
      if (typeof payload === 'number') {
        state.mouthTarget = payload;
      } else if (typeof payload?.level === 'number') {
        state.mouthTarget = payload.level;
      } else {
        state.mouthTarget = String(payload || '').toLowerCase() === 'speaking' ? 0.7 : 0;
      }
    });

    const eyeES = createEventSource('live2d.eye', (payload) => {
      if (Number.isFinite(Number(payload?.x))) state.targetX = Number(payload.x);
      if (Number.isFinite(Number(payload?.y))) state.targetY = Number(payload.y);
    });

    const reloadES = createEventSource('live2d.reload', async () => {
      await reloadCompanion();
    });

    state.eventSources.push(motionES, mouthES, eyeES, reloadES);
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
    destroyRenderer();
    connectEvents();
    setStatus('正在加载模型...');

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
    state.model.scale.set(Math.min(stage.clientWidth * 1.08 / state.model.width, stage.clientHeight * 1.22 / state.model.height));
    await state.model.motion('Idle', 0, 1).catch(() => undefined);

    state.app.ticker.add(() => {
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
    });

    setStatus('已就绪');
  }

  async function reloadCompanion() {
    setStatus('正在重载...');
    try {
      await initCompanion();
      setStatus('已就绪');
    } catch (err) {
      setStatus('重载失败');
      console.error(err);
    }
  }

  stage.addEventListener('mousemove', (evt) => {
    const r = stage.getBoundingClientRect();
    state.targetX = Math.max(-1, Math.min(1, ((evt.clientX - r.left) / Math.max(1, r.width) - 0.5) * 2));
    state.targetY = Math.max(-1, Math.min(1, ((evt.clientY - r.top) / Math.max(1, r.height) - 0.5) * 2));
  });
  stage.addEventListener('mouseleave', () => {
    state.targetX = 0;
    state.targetY = 0;
  });

  window.addEventListener('beforeunload', () => {
    destroyRenderer();
  });

  initCompanion().catch((err) => {
    setStatus('加载失败');
    console.error(err);
  });
}
