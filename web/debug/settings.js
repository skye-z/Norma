const api = 'http://127.0.0.1:8090';
const logEl = document.getElementById('log');
const providerLogEl = document.getElementById('provider_log');
const providerSummaryEl = document.getElementById('provider_summary');
const providerCatalogEl = document.getElementById('provider_catalog');
const providerAlertsEl = document.getElementById('provider_alerts');
const byId = (id) => document.getElementById(id);
const csv = (value) => String(value || '').split(',').map((item) => item.trim()).filter(Boolean);
const healthOrder = ['ready', 'partial', 'missing'];
const healthLabels = {
  binary: '二进制',
  model: '模型',
  voice_model: '语音模型',
  voice_config: '语音配置',
  encoder: 'encoder',
  decoder: 'decoder',
  joiner: 'joiner',
  tokens: 'tokens.txt',
  keywords: 'keywords.txt',
  ollama: 'Ollama'
};

let current = null;
let providerStatus = null;
let providerCatalog = null;
let healthFilter = 'all';

function writeLog(target, message) {
  if (!target) return;
  target.textContent = `[${new Date().toLocaleTimeString()}] ${message}\n${target.textContent}`;
}

function log(message) {
  writeLog(logEl, message);
}

function providerLog(message) {
  writeLog(providerLogEl, message);
}

function stateLabel(status) {
  switch (status) {
    case 'ready': return '就绪';
    case 'partial': return '部分就绪';
    default: return '缺失';
  }
}

function stateClass(status) {
  switch (status) {
    case 'ready': return 'state-pill';
    case 'partial': return 'state-pill warn';
    default: return 'state-pill danger';
  }
}

function displayAsset(key) {
  return healthLabels[key] || key;
}

function joinAssets(items) {
  return (items || []).map(displayAsset).join('、');
}

function healthText(health) {
  if (!health) return '未检测';
  if (!health.missing?.length) return `已满足：${joinAssets(health.found) || '全部'}`;
  if (!health.found?.length) return `缺少：${joinAssets(health.missing)}`;
  return `已找到：${joinAssets(health.found)}；缺少：${joinAssets(health.missing)}`;
}

function apply(cfg) {
  byId('llm_provider').value = cfg.llm?.provider || '';
  byId('llm_model').value = cfg.llm?.model || '';
  byId('llm_endpoint').value = cfg.llm?.endpoint || '';
  byId('llm_timeout').value = cfg.llm?.request_timeout_ms || 45000;
  byId('asr_provider').value = cfg.asr?.provider || '';
  byId('asr_binary_path').value = cfg.asr?.binary_path || '';
  byId('asr_model_path').value = cfg.asr?.model_path || '';
  byId('tts_provider').value = cfg.tts?.provider || '';
  byId('tts_voice').value = cfg.tts?.voice || '';
  byId('tts_binary_path').value = cfg.tts?.binary_path || '';
  byId('tts_model_path').value = cfg.tts?.model_path || '';
  byId('tts_sentence_first').checked = !!cfg.tts?.sentence_first_enabled;
  byId('audio_wake_mode').value = cfg.audio?.wake_mode || 'wake_word_vad';
  byId('audio_wake_provider').value = cfg.audio?.wake_provider || 'sherpa-onnx';
  byId('audio_wake_binary_path').value = cfg.audio?.wake_binary_path || '';
  byId('audio_wake_model_path').value = cfg.audio?.wake_model_path || '';
  byId('audio_wake_keywords_file_path').value = cfg.audio?.wake_keywords_file_path || '';
  byId('audio_wake_keyword').value = cfg.audio?.wake_keyword || '诺玛';
  byId('audio_wake_window_ms').value = cfg.audio?.wake_window_ms || 2000;
  byId('audio_wake_detect_interval_ms').value = cfg.audio?.wake_detect_interval_ms || 400;
  byId('audio_silence').value = cfg.audio?.endpoint_silence_frames || 4;
  byId('audio_interrupt').checked = !!cfg.audio?.interrupt_during_speaking;
  byId('audio_aec').checked = !!cfg.audio?.aec_enabled;
  byId('audio_ns').checked = !!cfg.audio?.ns_enabled;
  byId('audio_scene').value = cfg.audio?.scene || 'speaker';
  byId('rag_topk').value = cfg.rag?.top_k || 6;
  byId('rag_embed_provider').value = cfg.rag?.embed_provider || 'ollama';
  byId('rag_embed_endpoint').value = cfg.rag?.embed_endpoint || '';
  byId('rag_embed_model').value = cfg.rag?.embed_model || '';
  byId('stm_budget').value = cfg.memory?.stm_token_budget || 2048;
  byId('stm_keep_important').checked = !!cfg.memory?.stm_reserve_important;
  byId('memory_summary_auto').checked = !!cfg.memory?.summary_auto_write;
  byId('perf_auto').checked = !!cfg.perf?.auto_degrade;
  byId('perf_first').value = cfg.perf?.llm_first_token_threshold_ms || 1200;
  byId('perf_topk').value = cfg.perf?.degraded_rag_top_k || 4;
  byId('perf_ef_search').value = cfg.perf?.degraded_ef_search || 24;
  byId('perf_extra_factor').value = cfg.perf?.degraded_retriever_extra_factor || 1;
  byId('perf_fuse_factor').value = cfg.perf?.degraded_retriever_fuse_factor || 2;
  byId('perf_audio_chunk').value = cfg.perf?.degraded_audio_chunk_size || 4096;
  byId('perf_degraded_llm_model').value = cfg.perf?.degraded_llm_model || '';
  byId('perf_degraded_tts_voice').value = cfg.perf?.degraded_tts_voice || 'zh_CN-huayan-x_low';
  byId('perf_alert_llm').value = cfg.perf?.alert_llm_first_token_ms || 1500;
  byId('perf_alert_tts').value = cfg.perf?.alert_tts_first_chunk_ms || 1200;
  byId('perf_baseline_file').value = cfg.perf?.baseline_file || 'configs/perf-baseline.json';
  byId('providers_root').value = cfg.providers?.root || 'runtime/providers';
  byId('security_allow_remote').checked = !!cfg.security?.allow_remote_providers;
  byId('security_hosts').value = (cfg.security?.allowed_remote_hosts || []).join(',');
  byId('security_tools').value = (cfg.security?.tool_whitelist || []).join(',');
  byId('security_confirm').checked = !!cfg.security?.high_risk_confirm;
  byId('security_purge_proof_dir').value = cfg.security?.purge_proof_dir || 'diagnostics/purge-proofs';
}

function collect(base) {
  const cfg = structuredClone(base);
  cfg.llm.provider = byId('llm_provider').value.trim();
  cfg.llm.model = byId('llm_model').value.trim();
  cfg.llm.endpoint = byId('llm_endpoint').value.trim();
  cfg.llm.request_timeout_ms = Number(byId('llm_timeout').value || 45000);
  cfg.asr.provider = byId('asr_provider').value.trim();
  cfg.asr.binary_path = byId('asr_binary_path').value.trim();
  cfg.asr.model_path = byId('asr_model_path').value.trim();
  cfg.tts.provider = byId('tts_provider').value.trim();
  cfg.tts.voice = byId('tts_voice').value.trim();
  cfg.tts.binary_path = byId('tts_binary_path').value.trim();
  cfg.tts.model_path = byId('tts_model_path').value.trim();
  cfg.tts.sentence_first_enabled = !!byId('tts_sentence_first').checked;
  cfg.audio.wake_mode = byId('audio_wake_mode').value.trim();
  cfg.audio.wake_provider = byId('audio_wake_provider').value.trim();
  cfg.audio.wake_binary_path = byId('audio_wake_binary_path').value.trim();
  cfg.audio.wake_model_path = byId('audio_wake_model_path').value.trim();
  cfg.audio.wake_keywords_file_path = byId('audio_wake_keywords_file_path').value.trim();
  cfg.audio.wake_keyword = byId('audio_wake_keyword').value.trim();
  cfg.audio.wake_window_ms = Number(byId('audio_wake_window_ms').value || 2000);
  cfg.audio.wake_detect_interval_ms = Number(byId('audio_wake_detect_interval_ms').value || 400);
  cfg.audio.endpoint_silence_frames = Number(byId('audio_silence').value || 4);
  cfg.audio.interrupt_during_speaking = !!byId('audio_interrupt').checked;
  cfg.audio.aec_enabled = !!byId('audio_aec').checked;
  cfg.audio.ns_enabled = !!byId('audio_ns').checked;
  cfg.audio.scene = byId('audio_scene').value.trim();
  cfg.rag.top_k = Number(byId('rag_topk').value || 6);
  cfg.rag.embed_provider = byId('rag_embed_provider').value.trim();
  cfg.rag.embed_endpoint = byId('rag_embed_endpoint').value.trim();
  cfg.rag.embed_model = byId('rag_embed_model').value.trim();
  cfg.memory.stm_token_budget = Number(byId('stm_budget').value || 2048);
  cfg.memory.stm_reserve_important = !!byId('stm_keep_important').checked;
  cfg.memory.summary_auto_write = !!byId('memory_summary_auto').checked;
  cfg.perf.auto_degrade = !!byId('perf_auto').checked;
  cfg.perf.llm_first_token_threshold_ms = Number(byId('perf_first').value || 1200);
  cfg.perf.degraded_rag_top_k = Number(byId('perf_topk').value || 4);
  cfg.perf.degraded_ef_search = Number(byId('perf_ef_search').value || 24);
  cfg.perf.degraded_retriever_extra_factor = Number(byId('perf_extra_factor').value || 1);
  cfg.perf.degraded_retriever_fuse_factor = Number(byId('perf_fuse_factor').value || 2);
  cfg.perf.degraded_audio_chunk_size = Number(byId('perf_audio_chunk').value || 4096);
  cfg.perf.degraded_llm_model = byId('perf_degraded_llm_model').value.trim();
  cfg.perf.degraded_tts_voice = byId('perf_degraded_tts_voice').value.trim();
  cfg.perf.alert_llm_first_token_ms = Number(byId('perf_alert_llm').value || 1500);
  cfg.perf.alert_tts_first_chunk_ms = Number(byId('perf_alert_tts').value || 1200);
  cfg.perf.baseline_file = byId('perf_baseline_file').value.trim();
  cfg.providers.root = byId('providers_root').value.trim();
  cfg.security.allow_remote_providers = !!byId('security_allow_remote').checked;
  cfg.security.allowed_remote_hosts = csv(byId('security_hosts').value);
  cfg.security.tool_whitelist = csv(byId('security_tools').value);
  cfg.security.high_risk_confirm = !!byId('security_confirm').checked;
  cfg.security.purge_proof_dir = byId('security_purge_proof_dir').value.trim();
  return cfg;
}

function renderProviderAlerts(status) {
  providerAlertsEl.innerHTML = '';
  const entries = Object.entries(status.health || {});
  const partial = entries.filter(([, health]) => health.status === 'partial');
  const missing = entries.filter(([, health]) => health.status === 'missing');
  if (!partial.length && !missing.length) {
    const card = document.createElement('div');
    card.className = 'metric-tile';
    card.innerHTML = '<div class="metric-label">状态总览</div><div class="metric-foot">当前受管 Provider 已全部达到就绪状态。</div>';
    providerAlertsEl.appendChild(card);
    return;
  }
  for (const [kind, health] of [...missing, ...partial]) {
    const card = document.createElement('div');
    card.className = 'metric-tile';
    card.innerHTML = `<div class="metric-label">${kind.toUpperCase()}</div><div class="metric-foot"><span class="${stateClass(health.status)}">${stateLabel(health.status)}</span></div><div class="metric-foot">${healthText(health)}</div>`;
    providerAlertsEl.appendChild(card);
  }
}

function renderProviderSummary(status) {
  providerSummaryEl.innerHTML = '';
  const basics = [
    ['根目录', status.root || '-'],
    ['当前 Wake', status.stack?.wake || '-'],
    ['当前 ASR', status.stack?.asr || '-'],
    ['当前 TTS', status.stack?.tts || '-'],
    ['当前 Embedding', status.stack?.embedding || '-'],
    ['Manifest 记录数', String(status.manifest?.providers?.length || 0)]
  ];
  for (const [label, value] of basics) {
    const row = document.createElement('div');
    row.className = 'stat-line';
    row.innerHTML = `<span>${label}</span><strong>${value}</strong>`;
    providerSummaryEl.appendChild(row);
  }
  const healthEntries = Object.entries(status.health || {})
    .filter(([, health]) => healthFilter === 'all' || health.status === healthFilter)
    .sort((a, b) => healthOrder.indexOf(a[1]?.status || 'missing') - healthOrder.indexOf(b[1]?.status || 'missing'));
  for (const [kind, health] of healthEntries) {
    const row = document.createElement('div');
    row.className = 'stat-line';
    row.innerHTML = `<span>${kind}</span><div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap; justify-content:flex-end;"><span class="${stateClass(health.status)}">${stateLabel(health.status)}</span><span>${healthText(health)}</span></div>`;
    providerSummaryEl.appendChild(row);
  }
}

function renderProviderCatalog(catalog) {
  providerCatalogEl.innerHTML = '';
  const manifestMap = new Map((providerStatus?.manifest?.providers || []).map((item) => [item.id || item.name, item]));
  const installed = new Set(manifestMap.keys());
  const health = providerStatus?.health || {};
  for (const item of catalog.items || []) {
    const providerHealth = health[item.kind];
    if (healthFilter !== 'all' && providerHealth?.status !== healthFilter) continue;
    const record = manifestMap.get(item.id);
    const card = document.createElement('div');
    card.className = 'metric-tile';
    const installedText = installed.has(item.id) ? '已安装' : '未安装';
    const verify = item.integrity === 'official-checksum'
      ? '校验：已接入官方 SHA256 校验'
      : (item.integrity_note ? `校验：${item.integrity_note}` : '校验：未配置 SHA256 校验');
    card.innerHTML = `
      <div class="metric-label">${String(item.kind || '').toUpperCase()} / ${item.version || '-'}</div>
      <div class="metric-value" style="font-size:18px;">${item.name || item.id}</div>
      <div class="metric-foot">${item.id}</div>
      <div class="metric-foot">安装状态：${installedText}</div>
      <div class="metric-foot">${verify}</div>
      ${record?.local_sha256 ? `<div class="metric-foot">本地 SHA256：${record.local_sha256}</div>` : ''}
      ${record?.installed_at ? `<div class="metric-foot">安装时间：${record.installed_at}</div>` : ''}
      ${providerHealth ? `<div class="metric-foot">目录状态：<span class="${stateClass(providerHealth.status)}">${stateLabel(providerHealth.status)}</span></div><div class="metric-foot">${healthText(providerHealth)}</div>` : ''}
      ${item.notes ? `<div class="metric-foot">${item.notes}</div>` : ''}
      <div class="hero-actions" style="margin-top:12px;">
        <button data-install="${item.id}" class="primary">安装</button>
        <button data-uninstall="${item.id}" class="danger">卸载</button>
      </div>`;
    providerCatalogEl.appendChild(card);
  }
  providerCatalogEl.querySelectorAll('[data-install]').forEach((button) => {
    button.addEventListener('click', async () => installProvider(button.dataset.install));
  });
  providerCatalogEl.querySelectorAll('[data-uninstall]').forEach((button) => {
    button.addEventListener('click', async () => uninstallProvider(button.dataset.uninstall));
  });
}

function rerenderProviders() {
  if (providerStatus) {
    renderProviderAlerts(providerStatus);
    renderProviderSummary(providerStatus);
  }
  if (providerCatalog) renderProviderCatalog(providerCatalog);
}

async function load() {
  const response = await fetch(`${api}/settings/runtime`);
  if (!response.ok) throw new Error(`读取配置失败：${response.status}`);
  current = await response.json();
  apply(current);
  log('已读取当前运行时配置。');
}

async function loadProviderStatus() {
  const response = await fetch(`${api}/providers/status`);
  if (!response.ok) throw new Error(`读取 Provider 状态失败：${response.status}`);
  providerStatus = await response.json();
  rerenderProviders();
  providerLog('已刷新 Provider 状态。');
}

async function loadProviderCatalog() {
  const response = await fetch(`${api}/providers/catalog`);
  if (!response.ok) throw new Error(`读取 Provider 清单失败：${response.status}`);
  providerCatalog = await response.json();
  rerenderProviders();
}

async function save() {
  if (!current) return;
  const response = await fetch(`${api}/settings/runtime`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(collect(current)) });
  if (!response.ok) throw new Error(`保存配置失败：${response.status} ${await response.text()}`);
  log('配置保存成功，已广播运行时更新事件。');
  await load();
  await loadProviderStatus();
}

async function initProviders() {
  const response = await fetch(`${api}/providers/init`, { method: 'POST' });
  if (!response.ok) throw new Error(`初始化 Provider 目录失败：${response.status}`);
  providerLog('Provider 目录初始化完成。');
  await loadProviderStatus();
}

async function applyManagedProviders() {
  const response = await fetch(`${api}/providers/apply-managed`, { method: 'POST' });
  if (!response.ok) throw new Error(`应用受管 Provider 失败：${response.status}`);
  providerLog('已根据受管目录自动回填配置。');
  await load();
  await loadProviderStatus();
}

async function exportReadiness() {
  const response = await fetch(`${api}/providers/export-readiness`, { method: 'POST' });
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || `导出失败：${response.status}`);
  providerLog(`就绪检查报告已导出到 ${data.path}`);
}

async function installProvider(id) {
  providerLog(`开始安装 ${id} ...`);
  const response = await fetch(`${api}/providers/install`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id }) });
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || `安装失败：${response.status}`);
  providerLog(String(data.output || `${id} 安装成功`));
  await loadProviderStatus();
  await loadProviderCatalog();
}

async function uninstallProvider(id) {
  providerLog(`开始卸载 ${id} ...`);
  const response = await fetch(`${api}/providers/uninstall`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id }) });
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || `卸载失败：${response.status}`);
  providerLog(String(data.output || `${id} 卸载成功`));
  await loadProviderStatus();
  await loadProviderCatalog();
}

byId('load').addEventListener('click', () => Promise.all([load(), loadProviderStatus(), loadProviderCatalog()]).catch((error) => log(String(error))));
byId('save').addEventListener('click', () => save().catch((error) => log(String(error))));
byId('provider_init').addEventListener('click', () => initProviders().catch((error) => providerLog(String(error))));
byId('provider_apply').addEventListener('click', () => applyManagedProviders().catch((error) => providerLog(String(error))));
byId('provider_export').addEventListener('click', () => exportReadiness().catch((error) => providerLog(String(error))));
document.querySelectorAll('[data-health-filter]').forEach((button) => {
  button.addEventListener('click', () => {
    healthFilter = button.dataset.healthFilter || 'all';
    document.querySelectorAll('[data-health-filter]').forEach((item) => item.classList.remove('primary'));
    button.classList.add('primary');
    rerenderProviders();
  });
});

Promise.all([load(), loadProviderStatus(), loadProviderCatalog()]).catch((error) => {
  log(String(error));
  providerLog(String(error));
});