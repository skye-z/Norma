const api = 'http://127.0.0.1:8090';
const countersEl = document.getElementById('counters');
const recentEl = document.getElementById('recent');
const alertsEl = document.getElementById('alerts');
const counterCountEl = document.getElementById('counterCount');
const recentCountEl = document.getElementById('recentCount');
const alertCountEl = document.getElementById('alertCount');

async function loadMetrics() {
  const resp = await fetch(`${api}/metrics/summary`);
  if (!resp.ok) throw new Error(`读取指标失败: ${resp.status}`);
  const data = await resp.json();
  const counters = data.counters || [];
  const recent = data.recent || [];
  const alerts = data.alerts || [];
  countersEl.textContent = counters.map((item) => `${item.name}\n  次数: ${item.count}\n  最近值: ${item.last_value}`).join('\n\n') || '暂无计数器数据';
  recentEl.textContent = JSON.stringify(recent, null, 2) || '[]';
  alertsEl.textContent = alerts.join('\n') || '暂无告警';
  counterCountEl.textContent = String(counters.length);
  recentCountEl.textContent = String(recent.length);
  alertCountEl.textContent = String(alerts.length);
}

document.getElementById('refresh')?.addEventListener('click', () => {
  loadMetrics().catch((e) => {
    countersEl.textContent = String(e);
  });
});

document.getElementById('export')?.addEventListener('click', () => {
  window.open(`${api}/metrics/export`, '_blank');
});

setInterval(() => loadMetrics().catch(() => {}), 3000);
loadMetrics().catch((e) => {
  countersEl.textContent = String(e);
});