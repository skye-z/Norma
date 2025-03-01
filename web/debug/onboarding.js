const api = 'http://127.0.0.1:8090';
const summaryEl = document.getElementById('summary');
const checksEl = document.getElementById('checks');
const suggestionsEl = document.getElementById('suggestions');
const fallbackModesEl = document.getElementById('fallback_modes');
const logEl = document.getElementById('log');

function pill(status) {
  const cls = status === 'ready' ? 'state-pill' : status === 'partial' ? 'state-pill warn' : 'state-pill danger';
  const label = status === 'ready' ? '就绪' : status === 'partial' ? '部分就绪' : '缺失';
  return `<span class="${cls}">${label}</span>`;
}

function log(message) {
  logEl.textContent = `[${new Date().toLocaleTimeString()}] ${message}\n${logEl.textContent}`;
}

function render(report) {
  summaryEl.innerHTML = '';
  const lines = [
    ['总体状态', report.ready ? '可以进入桌面伴侣模式' : '仍需补齐依赖'],
    ['Wake', report.provider?.wake?.status || '-'],
    ['ASR', report.provider?.asr?.status || '-'],
    ['TTS', report.provider?.tts?.status || '-'],
    ['Live2D', report.model?.issues?.length ? '缺失' : '就绪']
  ];
  for (const [label, value] of lines) {
    const row = document.createElement('div');
    row.className = 'stat-line';
    row.innerHTML = `<span>${label}</span><strong>${value}</strong>`;
    summaryEl.appendChild(row);
  }

  checksEl.innerHTML = '';
  for (const check of report.checks || []) {
    const card = document.createElement('div');
    card.className = 'metric-tile';
    card.innerHTML = `
      <div class="metric-label">${check.name}</div>
      <div class="metric-foot">${pill(check.status)}</div>
      <div class="metric-foot">${check.detail}</div>
      <div class="metric-foot">建议：${check.hint}</div>
    `;
    checksEl.appendChild(card);
  }

  suggestionsEl.innerHTML = '';
  for (const item of report.suggestions || []) {
    const card = document.createElement('div');
    card.className = 'metric-tile';
    card.innerHTML = `<div class="metric-foot">${item}</div>`;
    suggestionsEl.appendChild(card);
  }

  fallbackModesEl.innerHTML = '';
  for (const item of report.fallback_modes || []) {
    const card = document.createElement('div');
    card.className = 'metric-tile';
    card.innerHTML = `<div class="metric-foot">${item}</div>`;
    fallbackModesEl.appendChild(card);
  }
}

async function load() {
  const resp = await fetch(`${api}/setup/wizard`);
  if (!resp.ok) {
    throw new Error(`读取首启向导失败：${resp.status}`);
  }
  const data = await resp.json();
  render(data);
  log('首启向导检查完成。');
}

document.getElementById('refresh').addEventListener('click', () => load().catch((err) => log(String(err))));
load().catch((err) => log(String(err)));
