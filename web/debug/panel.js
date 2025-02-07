const api = 'http://127.0.0.1:8090';
const logEl = document.getElementById('log');
const log = (s) => { if (logEl) logEl.textContent = `[${new Date().toLocaleTimeString()}] ${s}\n` + logEl.textContent; };
const pub = (name, payload) => fetch(`${api}/runtime/publish`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, payload }) });

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

const es = new EventSource(`${api}/runtime/events?name=runtime.config.updated`);
es.onmessage = (e) => log(`配置已更新: ${e.data}`);