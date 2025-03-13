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

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function formatTs(ts) {
  if (!ts) return '-';
  return new Date(ts).toLocaleTimeString();
}

function summarizeEvent(evt) {
  const payload = evt?.payload || {};
  switch (evt?.name) {
    case 'live2d.eye':
      return {
        title: 'Live2D 视线跟随',
        detail: `视线移动到 X=${Number(payload.x ?? 0).toFixed(2)} / Y=${Number(payload.y ?? 0).toFixed(2)}`
      };
    case 'live2d.motion':
      return {
        title: 'Live2D 动作触发',
        detail: `动作组 ${payload.group || '-'}，索引 ${payload.index ?? 0}`
      };
    case 'audio.vad.start':
      return {
        title: '语音打断开始',
        detail: '收到 VAD 开始事件，打断链路被触发'
      };
    case 'asr.partial':
      return {
        title: 'ASR Partial',
        detail: `识别中内容：${typeof payload === 'string' ? payload : payload.text || '-'}`
      };
    case 'asr.final':
      return {
        title: 'ASR Final',
        detail: `最终识别内容：${typeof payload === 'string' ? payload : payload.text || '-'}`
      };
    default:
      return {
        title: evt?.name || '未知事件',
        detail: buildPayloadSummary(payload)
      };
  }
}

function buildPayloadSummary(payload) {
  if (payload == null) return '无附加信息';
  if (typeof payload === 'string') return payload;
  if (typeof payload !== 'object') return String(payload);
  const entries = Object.entries(payload).slice(0, 4).map(([key, value]) => `${key}: ${typeof value === 'object' ? '[对象]' : String(value)}`);
  return entries.join(' / ') || '无附加信息';
}

function renderEvents(events) {
  if (!eventsEl) return;
  const filtered = (events || []).filter((evt) => evt?.name !== 'live2d.eye').slice(-8).reverse();
  if (!filtered.length) {
    eventsEl.innerHTML = '<div class="event-item"><div class="event-title">暂无关键事件</div><div class="event-detail">高频背景事件默认已隐藏。</div></div>';
    return;
  }
  eventsEl.innerHTML = filtered.map((evt) => {
    const summary = summarizeEvent(evt);
    return `
      <div class="event-item">
        <div class="event-head">
          <div class="event-title">${escapeHtml(summary.title)}</div>
          <div class="event-time">${escapeHtml(formatTs(evt.ts_unix_ms))}</div>
        </div>
        <div class="event-detail">${escapeHtml(summary.detail)}</div>
      </div>
    `;
  }).join('');
}

function showResult(data) {
  if (!resultEl) return;
  const details = Object.entries(data?.details || {}).map(([key, value]) => `
    <div class="stat-line">
      <span>${escapeHtml(key)}</span>
      <strong>${escapeHtml(typeof value === 'object' ? JSON.stringify(value) : String(value))}</strong>
    </div>
  `).join('');
  resultEl.innerHTML = `
    <div class="result-card">
      <div class="result-head">
        <div class="result-title">${escapeHtml(data?.target || '联调结果')}</div>
        <span class="state-pill ${data?.ok ? '' : 'danger'}">${data?.ok ? '正常' : '异常'}</span>
      </div>
      <div class="result-detail">${escapeHtml(data?.summary || '-')}</div>
      <div class="result-meta">当前状态：${escapeHtml(data?.state || '-')}</div>
      ${details || '<div class="result-detail">没有更多细节</div>'}
    </div>
  `;
}

async function refreshDiagnostics() {
  const resp = await fetch(`${api}/runtime/diagnostics?limit=20`);
  if (!resp.ok) throw new Error(`读取诊断失败: ${resp.status}`);
  const data = await resp.json();
  stateEl.textContent = data.state || '-';
  lastEventEl.textContent = data.last_event || '-';
  queueDepthEl.textContent = String(data.queue_depth ?? 0);
  sessionCountEl.textContent = String((data.sessions || []).length);
  renderEvents(data.events || []);
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
