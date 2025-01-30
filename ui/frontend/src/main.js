import './style.css';

const eventApiBase = 'http://127.0.0.1:8090';

document.querySelector('#app').innerHTML = `
  <main class="page">
    <section class="panel">
      <h1>Norma Runtime Monitor</h1>
      <p class="hint">Streaming from <code>/runtime/events</code></p>
      <div class="row"><span>Motion:</span> <strong id="motion">-</strong></div>
      <div class="row"><span>Mouth:</span> <strong id="mouth">-</strong></div>
      <div class="row"><span>Status:</span> <strong id="status">connecting...</strong></div>
      <button id="refresh" class="btn">Reconnect</button>
    </section>
    <section class="panel logs">
      <h2>Event Log</h2>
      <pre id="log"></pre>
    </section>
  </main>
`;

const motionEl = document.getElementById('motion');
const mouthEl = document.getElementById('mouth');
const statusEl = document.getElementById('status');
const logEl = document.getElementById('log');
const refreshBtn = document.getElementById('refresh');

let closers = [];

function appendLog(line) {
  const ts = new Date().toLocaleTimeString();
  logEl.textContent = `[${ts}] ${line}\n` + logEl.textContent;
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
    connectEvent('live2d.motion', (evt) => {
      motionEl.textContent = String(evt.payload ?? '-');
    })
  );

  closers.push(
    connectEvent('live2d.mouth', (evt) => {
      mouthEl.textContent = String(evt.payload ?? '-');
    })
  );
}

refreshBtn.addEventListener('click', connectAll);
connectAll();
