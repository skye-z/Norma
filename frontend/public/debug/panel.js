const api = 'http://127.0.0.1:8090';

const logEl = document.getElementById('log');
const eventsEl = document.getElementById('events');
const resultEl = document.getElementById('result');
const stateEl = document.getElementById('state');
const lastEventEl = document.getElementById('lastEvent');
const queueDepthEl = document.getElementById('queueDepth');
const sessionCountEl = document.getElementById('sessionCount');

function log(message) {
  if (!logEl) return;
  logEl.textContent = `[${new Date().toLocaleTimeString()}] ${message}\n${logEl.textContent}`;
}

function showResult(data) {
  if (!resultEl) return;
  resultEl.textContent = JSON.stringify(data, null, 2);
}

async function refreshDiagnostics() {
  const resp = await fetch(`${api}/runtime/diagnostics?limit=20`);
  if (!resp.ok) throw new Error(`读取诊断失败: ${resp.status}`);
  const data = await resp.json();
  stateEl.textContent = data.state || '-';
  lastEventEl.textContent = data.last_event || '-';
  queueDepthEl.textContent = String(data.queue_depth ?? 0);
  sessionCountEl.textContent = String((data.sessions || []).length);
  const lines = (data.events || []).map((evt) => JSON.stringify(evt, null, 2));
  eventsEl.textContent = lines.join('\n\n') || '暂无事件';
}

async function runDiagnostic(target, payload = {}) {
  const response = await fetch(`${api}/diagnostics/run/${target}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  const data = await response.json();
  if (!response.ok) {
    throw new Error(data?.error?.message || `联调失败: ${response.status}`);
  }
  showResult(data);
  log(`${target} 联调完成: ${data.summary}`);
  await refreshDiagnostics();
  return data;
}

document.querySelectorAll('.diag-run').forEach((button) => {
  button.addEventListener('click', async () => {
    try {
      await runDiagnostic('live2d', {
        motion: button.dataset.motion || 'Tap',
        eye_x: Number(document.getElementById('ex')?.value || 0),
        eye_y: Number(document.getElementById('ey')?.value || 0)
      });
    } catch (error) {
      log(String(error));
    }
  });
});

document.getElementById('run_tts')?.addEventListener('click', async () => {
  try {
    await runDiagnostic('tts', { text: String(document.getElementById('txt')?.value || '').trim() });
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('run_asr_partial')?.addEventListener('click', async () => {
  try {
    await runDiagnostic('asr', { text: String(document.getElementById('txt')?.value || '').trim(), stage: 'partial' });
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('run_asr_final')?.addEventListener('click', async () => {
  try {
    await runDiagnostic('asr', { text: String(document.getElementById('txt')?.value || '').trim(), stage: 'final' });
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('run_llm')?.addEventListener('click', async () => {
  try {
    await runDiagnostic('llm', { text: String(document.getElementById('probe_text')?.value || '').trim() });
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('run_memory')?.addEventListener('click', async () => {
  try {
    await runDiagnostic('memory', { text: String(document.getElementById('probe_text')?.value || '').trim() });
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('run_rag')?.addEventListener('click', async () => {
  try {
    await runDiagnostic('rag', { text: String(document.getElementById('probe_text')?.value || '').trim() });
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('run_reload')?.addEventListener('click', async () => {
  try {
    await runDiagnostic('live2d', {
      motion: 'Reload',
      eye_x: Number(document.getElementById('ex')?.value || 0),
      eye_y: Number(document.getElementById('ey')?.value || 0)
    });
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('mouth')?.addEventListener('input', () => {
  log(`口型滑杆预设为 ${document.getElementById('mouth')?.value || 0}，请结合 Live2D 联调按钮观察效果`);
});

document.getElementById('ex')?.addEventListener('input', () => {
  log(`视线 X 调整为 ${document.getElementById('ex')?.value || 0}`);
});

document.getElementById('ey')?.addEventListener('input', () => {
  log(`视线 Y 调整为 ${document.getElementById('ey')?.value || 0}`);
});

document.getElementById('bargein')?.addEventListener('click', async () => {
  try {
    await fetch(`${api}/runtime/publish`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'audio.vad.start', session_id: 'debug-panel', trace_id: 'debug-trace', payload: { source: 'diagnostics-ui' } })
    });
    log('已发送模拟打断事件');
    await refreshDiagnostics();
  } catch (error) {
    log(String(error));
  }
});

document.getElementById('refresh')?.addEventListener('click', () => {
  refreshDiagnostics().catch((error) => log(String(error)));
});

setInterval(() => {
  refreshDiagnostics().catch((error) => log(String(error)));
}, 3000);

refreshDiagnostics().catch((error) => log(String(error)));
