const api = 'http://127.0.0.1:8090';
const logEl = document.getElementById('log');
const eventsEl = document.getElementById('events');
const stateEl = document.getElementById('state');
const lastEventEl = document.getElementById('lastEvent');
const queueDepthEl = document.getElementById('queueDepth');
const sessionCountEl = document.getElementById('sessionCount');

const log = (s) => {
  if (logEl) logEl.textContent = `[${new Date().toLocaleTimeString()}] ${s}\n` + logEl.textContent;
};

const pub = (name, payload) => fetch(`${api}/runtime/publish`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ name, session_id: 'debug-panel', trace_id: 'debug-trace', payload })
});

async function refreshDiagnostics() {
  const resp = await fetch(`${api}/runtime/diagnostics?limit=20`);
  if (!resp.ok) throw new Error(`diagnostics failed: ${resp.status}`);
  const data = await resp.json();
  stateEl.textContent = data.state || '-';
  lastEventEl.textContent = data.last_event || '-';
  queueDepthEl.textContent = String(data.queue_depth ?? 0);
  sessionCountEl.textContent = String((data.sessions || []).length);
  eventsEl.textContent = (data.events || []).map((evt) => JSON.stringify(evt, null, 2)).join('\n\n');
}

document.querySelectorAll('.m').forEach((b) => b.addEventListener('click', async () => {
  await pub('live2d.motion', { group: b.dataset.m, index: 0 });
  log(`动作 -> ${b.dataset.m}`);
}));

document.getElementById('mouth')?.addEventListener('input', async (e) => {
  await pub('live2d.mouth', { level: Number(e.target.value) });
});

const emitEye = async () => {
  const x = Number(document.getElementById('ex')?.value || 0);
  const y = Number(document.getElementById('ey')?.value || 0);
  await pub('live2d.eye', { x, y });
};
document.getElementById('ex')?.addEventListener('input', emitEye);
document.getElementById('ey')?.addEventListener('input', emitEye);

document.getElementById('speak')?.addEventListener('click', () => {
  const text = String(document.getElementById('txt')?.value || '').trim();
  if (!text) return;
  const u = new SpeechSynthesisUtterance(text);
  u.lang = 'zh-CN';
  const timer = setInterval(() => pub('live2d.mouth', { level: 0.2 + Math.random() * 0.8 }), 90);
  u.onend = () => { clearInterval(timer); pub('live2d.mouth', { level: 0 }); };
  u.onerror = u.onend;
  speechSynthesis.cancel();
  speechSynthesis.speak(u);
});

document.getElementById('bargein')?.addEventListener('click', async () => {
  await pub('audio.vad.start', { source: 'debug-panel' });
  log('已发送模拟插话事件');
  await refreshDiagnostics();
});

document.getElementById('refresh')?.addEventListener('click', () => {
  refreshDiagnostics().catch((e) => log(String(e)));
});

const es = new EventSource(`${api}/runtime/events?name=runtime.config.updated`);
es.onmessage = (e) => log(`配置已更新: ${e.data}`);

setInterval(() => {
  refreshDiagnostics().catch((e) => log(String(e)));
}, 3000);
refreshDiagnostics().catch((e) => log(String(e)));