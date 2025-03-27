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

function normalizeEndpoint(value, fallback = '') {
  let endpoint = String(value || '').trim();
  if (!endpoint) endpoint = fallback;
  return endpoint.replace(/\/+$/, '');
}

function toModelName(item) {
  const candidate = item?.name || item?.id || item?.model || '';
  return String(candidate || '').trim();
}

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

function setInput(id, value, fallback = '') {
  const el = byId(id);
  if (!el) return;
  el.value = value ?? fallback;
}

function setCheck(id, value) {
  const el = byId(id);
  if (!el) return;
  el.checked = !!value;
}

function getValue(id, fallback = '') {
  const el = byId(id);
  if (!el) return fallback;
  return String(el.value || fallback).trim();
}

function getNumber(id, fallback) {
  const raw = getValue(id, '');
  const num = Number(raw);
  return Number.isFinite(num) ? num : fallback;
}

function getChecked(id) {
  const el = byId(id);
  return !!el?.checked;
}

function renderModelOptions(names, selected) {
  const select = byId('llm_model');
  if (!select) return;
  const unique = Array.from(new Set((names || []).map((name) => String(name || '').trim()).filter(Boolean)));
  const picked = String(selected || '').trim();
  if (picked && !unique.includes(picked)) unique.unshift(picked);
  if (!unique.length) unique.push('');
  select.innerHTML = unique.map((name) => `<option value="${name}">${name || '（未发现模型）'}</option>`).join('');
  select.value = picked || unique[0] || '';
}

async function loadLLMModels() {
  const endpoint = normalizeEndpoint(getValue('llm_endpoint'), 'http://127.0.0.1:11434');
  const selected = getValue('llm_model');
  const attempts = [
    `${endpoint}/api/tags`,
    `${endpoint}/v1/models`,
    `${endpoint}/models`
  ];

  for (const url of attempts) {
    try {
      const resp = await fetch(url);
      if (!resp.ok) continue;
      const data = await resp.json();
      const list = Array.isArray(data?.models) ? data.models : (Array.isArray(data?.data) ? data.data : []);
      const names = list.map(toModelName).filter(Boolean);
      if (names.length) {
        renderModelOptions(names, selected);
        log(`LLM 模型列表已更新：${names.length} 项（${url}）`);
        return;
      }
    } catch (_) {
      // try next path
    }
  }

  renderModelOptions([], selected);
  log(`加载 LLM 模型列表失败：${endpoint}`);
}

function apply(cfg) {
  setCheck('audio_enable_wake_word', cfg.audio?.enable_wake_word ?? true);
  setInput('audio_wake_mode', cfg.audio?.wake_mode || 'wake_word_vad');
  setInput('audio_wake_provider', cfg.audio?.wake_provider || 'sherpa-onnx');
  setInput('audio_wake_binary_path', cfg.audio?.wake_binary_path || '');
  setInput('audio_wake_model_path', cfg.audio?.wake_model_path || '');
  setInput('audio_wake_keywords_file_path', cfg.audio?.wake_keywords_file_path || '');
  setInput('audio_wake_keyword', cfg.audio?.wake_keyword || '诺玛');
  setInput('audio_wake_window_ms', cfg.audio?.wake_window_ms || 2000);
  setInput('audio_wake_detect_interval_ms', cfg.audio?.wake_detect_interval_ms || 400);
  setInput('audio_silence', cfg.audio?.endpoint_silence_frames || 4);
  setCheck('audio_interrupt', cfg.audio?.interrupt_during_speaking ?? true);
  setCheck('audio_aec', cfg.audio?.aec_enabled || false);
  setCheck('audio_ns', cfg.audio?.ns_enabled || false);
  setInput('audio_scene', cfg.audio?.scene || 'speaker');
  setInput('audio_mic_device_id', cfg.audio?.mic_device_id || 'default');
  setInput('audio_mic_sample_rate', cfg.audio?.mic_sample_rate || 16000);
  setInput('audio_mic_channel_count', cfg.audio?.mic_channel_count || 1);

  setInput('asr_provider', cfg.asr?.provider || 'whispercpp');
  setInput('asr_timeout', cfg.asr?.request_timeout_ms || 10000);
  setInput('asr_endpoint', cfg.asr?.endpoint || '');
  setInput('asr_stream_endpoint', cfg.asr?.stream_endpoint || '');
  setInput('asr_binary_path', cfg.asr?.binary_path || '');
  setInput('asr_model_path', cfg.asr?.model_path || '');
  setInput('asr_language', cfg.asr?.language || 'zh');
  setInput('asr_threads', cfg.asr?.threads || 4);

  setInput('llm_provider', cfg.llm?.provider || 'ollama');
  setInput('llm_endpoint', normalizeEndpoint(cfg.llm?.endpoint, 'http://127.0.0.1:11434'));
  setInput('llm_timeout', cfg.llm?.request_timeout_ms || 45000);
  renderModelOptions([], cfg.llm?.model || '');

  setInput('rag_embed_provider', cfg.rag?.embed_provider || 'ollama');
  setInput('rag_embed_endpoint', normalizeEndpoint(cfg.rag?.embed_endpoint, normalizeEndpoint(cfg.llm?.endpoint, 'http://127.0.0.1:11434')));
  setInput('rag_embed_model', cfg.rag?.embed_model || '');

  setInput('tts_provider', cfg.tts?.provider || 'piper');
  setInput('tts_timeout', cfg.tts?.request_timeout_ms || 30000);
  setInput('tts_voice', cfg.tts?.voice || '');
  setInput('tts_endpoint', cfg.tts?.endpoint || '');
  setInput('tts_binary_path', cfg.tts?.binary_path || '');
  setInput('tts_model_path', cfg.tts?.model_path || '');
  setCheck('tts_sentence_first', cfg.tts?.sentence_first_enabled ?? true);

  setInput('rag_topk', cfg.rag?.top_k || 6);
  setInput('rag_vector_index_path', cfg.rag?.vector_index_path || 'resource/index/vector.index');
  setInput('stm_budget', cfg.memory?.stm_token_budget || 2048);
  setInput('memory_compact_hours', cfg.memory?.governance_compact_hours || 24);
  setCheck('stm_keep_important', cfg.memory?.stm_reserve_important ?? true);
  setCheck('memory_summary_auto', cfg.memory?.summary_auto_write ?? true);

  setCheck('perf_auto', cfg.perf?.auto_degrade ?? true);
  setInput('perf_first', cfg.perf?.llm_first_token_threshold_ms || 1200);
  setInput('perf_topk', cfg.perf?.degraded_rag_top_k || 4);
  setInput('perf_ef_search', cfg.perf?.degraded_ef_search || 24);
  setInput('perf_extra_factor', cfg.perf?.degraded_retriever_extra_factor || 1);
  setInput('perf_fuse_factor', cfg.perf?.degraded_retriever_fuse_factor || 2);
  setInput('perf_audio_chunk', cfg.perf?.degraded_audio_chunk_size || 4096);
  setInput('perf_degraded_llm_model', cfg.perf?.degraded_llm_model || '');
  setInput('perf_degraded_tts_voice', cfg.perf?.degraded_tts_voice || 'zh_CN-huayan-x_low');
  setInput('perf_alert_llm', cfg.perf?.alert_llm_first_token_ms || 1500);
  setInput('perf_alert_tts', cfg.perf?.alert_tts_first_chunk_ms || 1200);
  setInput('perf_baseline_file', cfg.perf?.baseline_file || 'build/perf-baseline.json');

  setInput('providers_root', cfg.providers?.root || 'resource/runtime/providers');
  setCheck('security_allow_remote', cfg.security?.allow_remote_providers || false);
  setInput('security_hosts', (cfg.security?.allowed_remote_hosts || []).join(','));
  setInput('security_tools', (cfg.security?.tool_whitelist || []).join(','));
  setCheck('security_confirm', cfg.security?.high_risk_confirm ?? true);
  setInput('security_purge_proof_dir', cfg.security?.purge_proof_dir || 'resource/diagnostics/purge-proofs');
}

function collect(base) {
  const cfg = structuredClone(base);

  cfg.audio.enable_wake_word = getChecked('audio_enable_wake_word');
  cfg.audio.wake_mode = getValue('audio_wake_mode', 'wake_word_vad');
  cfg.audio.wake_provider = getValue('audio_wake_provider', 'sherpa-onnx');
  cfg.audio.wake_binary_path = getValue('audio_wake_binary_path');
  cfg.audio.wake_model_path = getValue('audio_wake_model_path');
  cfg.audio.wake_keywords_file_path = getValue('audio_wake_keywords_file_path');
  cfg.audio.wake_keyword = getValue('audio_wake_keyword', '诺玛');
  cfg.audio.wake_window_ms = getNumber('audio_wake_window_ms', 2000);
  cfg.audio.wake_detect_interval_ms = getNumber('audio_wake_detect_interval_ms', 400);
  cfg.audio.endpoint_silence_frames = getNumber('audio_silence', 4);
  cfg.audio.interrupt_during_speaking = getChecked('audio_interrupt');
  cfg.audio.aec_enabled = getChecked('audio_aec');
  cfg.audio.ns_enabled = getChecked('audio_ns');
  cfg.audio.scene = getValue('audio_scene', 'speaker');
  cfg.audio.mic_device_id = getValue('audio_mic_device_id', 'default');
  cfg.audio.mic_sample_rate = getNumber('audio_mic_sample_rate', 16000);
  cfg.audio.mic_channel_count = getNumber('audio_mic_channel_count', 1);

  cfg.asr.provider = getValue('asr_provider', 'whispercpp');
  cfg.asr.request_timeout_ms = getNumber('asr_timeout', 10000);
  cfg.asr.endpoint = getValue('asr_endpoint');
  cfg.asr.stream_endpoint = getValue('asr_stream_endpoint');
  cfg.asr.binary_path = getValue('asr_binary_path');
  cfg.asr.model_path = getValue('asr_model_path');
  cfg.asr.language = getValue('asr_language', 'zh');
  cfg.asr.threads = getNumber('asr_threads', 4);

  cfg.llm.provider = getValue('llm_provider', 'ollama');
  cfg.llm.endpoint = normalizeEndpoint(getValue('llm_endpoint'), 'http://127.0.0.1:11434');
  cfg.llm.request_timeout_ms = getNumber('llm_timeout', 45000);
  cfg.llm.model = getValue('llm_model');

  cfg.rag.embed_provider = getValue('rag_embed_provider', 'ollama');
  cfg.rag.embed_endpoint = normalizeEndpoint(getValue('rag_embed_endpoint'), cfg.llm.endpoint);
  cfg.rag.embed_model = getValue('rag_embed_model');

  cfg.tts.provider = getValue('tts_provider', 'piper');
  cfg.tts.request_timeout_ms = getNumber('tts_timeout', 30000);
  cfg.tts.voice = getValue('tts_voice');
  cfg.tts.endpoint = getValue('tts_endpoint');
  cfg.tts.binary_path = getValue('tts_binary_path');
  cfg.tts.model_path = getValue('tts_model_path');
  cfg.tts.sentence_first_enabled = getChecked('tts_sentence_first');

  cfg.rag.top_k = getNumber('rag_topk', 6);
  cfg.rag.vector_index_path = getValue('rag_vector_index_path');

  cfg.memory.stm_token_budget = getNumber('stm_budget', 2048);
  cfg.memory.governance_compact_hours = getNumber('memory_compact_hours', 24);
  cfg.memory.stm_reserve_important = getChecked('stm_keep_important');
  cfg.memory.summary_auto_write = getChecked('memory_summary_auto');

  cfg.perf.auto_degrade = getChecked('perf_auto');
  cfg.perf.llm_first_token_threshold_ms = getNumber('perf_first', 1200);
  cfg.perf.degraded_rag_top_k = getNumber('perf_topk', 4);
  cfg.perf.degraded_ef_search = getNumber('perf_ef_search', 24);
  cfg.perf.degraded_retriever_extra_factor = getNumber('perf_extra_factor', 1);
  cfg.perf.degraded_retriever_fuse_factor = getNumber('perf_fuse_factor', 2);
  cfg.perf.degraded_audio_chunk_size = getNumber('perf_audio_chunk', 4096);
  cfg.perf.degraded_llm_model = getValue('perf_degraded_llm_model');
  cfg.perf.degraded_tts_voice = getValue('perf_degraded_tts_voice', 'zh_CN-huayan-x_low');
  cfg.perf.alert_llm_first_token_ms = getNumber('perf_alert_llm', 1500);
  cfg.perf.alert_tts_first_chunk_ms = getNumber('perf_alert_tts', 1200);
  cfg.perf.baseline_file = getValue('perf_baseline_file', 'build/perf-baseline.json');

  cfg.providers.root = getValue('providers_root');
  cfg.security.allow_remote_providers = getChecked('security_allow_remote');
  cfg.security.allowed_remote_hosts = csv(getValue('security_hosts'));
  cfg.security.tool_whitelist = csv(getValue('security_tools'));
  cfg.security.high_risk_confirm = getChecked('security_confirm');
  cfg.security.purge_proof_dir = getValue('security_purge_proof_dir');

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

function buildAuditSummary() {
  if (!providerStatus) return 'Provider 状态尚未加载。';
  const lines = [];
  lines.push(`导出时间：${new Date().toISOString()}`);
  lines.push(`Provider 根目录：${providerStatus.root || '-'}`);
  lines.push(`当前栈：Wake=${providerStatus.stack?.wake || '-'} | ASR=${providerStatus.stack?.asr || '-'} | TTS=${providerStatus.stack?.tts || '-'} | Embedding=${providerStatus.stack?.embedding || '-'} | LLM=${providerStatus.stack?.llm || '-'}`);
  lines.push('');
  lines.push('目录就绪状态：');
  for (const [kind, health] of Object.entries(providerStatus.health || {})) {
    lines.push(`- ${kind}: ${stateLabel(health.status)}；${healthText(health)}`);
  }
  lines.push('');
  lines.push('安装来源审计：');
  const records = providerStatus.manifest?.providers || [];
  if (!records.length) {
    lines.push('- 当前没有已记录的受管 Provider。');
  } else {
    for (const record of records) {
      lines.push(`- ${record.kind || '-'} / ${record.name || record.id || '-'}`);
      lines.push(`  版本：${record.version || '-'}`);
      lines.push(`  完整性：${record.integrity || '-'}`);
      lines.push(`  说明：${record.integrity_note || '-'}`);
      lines.push(`  来源：${record.source || '-'}`);
      lines.push(`  本地 SHA256：${record.local_sha256 || '-'}`);
      lines.push(`  安装时间：${record.installed_at || '-'}`);
    }
  }
  return lines.join('\n');
}

async function copyAuditSummary() {
  const text = buildAuditSummary();
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
  } else {
    const area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', 'readonly');
    area.style.position = 'fixed';
    area.style.left = '-9999px';
    document.body.appendChild(area);
    area.select();
    document.execCommand('copy');
    document.body.removeChild(area);
  }
  providerLog('来源审计摘要已复制到剪贴板。');
}

async function openReadinessDir() {
  const response = await fetch(`${api}/providers/open-readiness-dir`, { method: 'POST' });
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || `打开目录失败：${response.status}`);
  providerLog(`已打开导出目录：${data.path}`);
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
  await loadLLMModels();
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
  const response = await fetch(`${api}/settings/runtime`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(collect(current))
  });
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
  const response = await fetch(`${api}/providers/install`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id })
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || `安装失败：${response.status}`);
  providerLog(String(data.output || `${id} 安装成功`));
  await loadProviderStatus();
  await loadProviderCatalog();
}

async function uninstallProvider(id) {
  providerLog(`开始卸载 ${id} ...`);
  const response = await fetch(`${api}/providers/uninstall`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id })
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || `卸载失败：${response.status}`);
  providerLog(String(data.output || `${id} 卸载成功`));
  await loadProviderStatus();
  await loadProviderCatalog();
}

async function testMicrophone() {
  const response = await fetch(`${api}/conversation/mic/test`, { method: 'POST' });
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || `麦克风测试失败：${response.status}`);
  const mic = data.mic || {};
  log(`${data.summary}（设备=${mic.device_id || '-'}，采样率=${mic.sample_rate || '-'}，声道=${mic.channel_count || '-'}，speech_frames=${mic.speech_frames || 0}）`);
}

byId('load')?.addEventListener('click', () => Promise.all([load(), loadProviderStatus(), loadProviderCatalog()]).catch((error) => log(String(error))));
byId('save')?.addEventListener('click', () => save().catch((error) => log(String(error))));
byId('audio_mic_test')?.addEventListener('click', () => testMicrophone().catch((error) => log(String(error))));
byId('refresh_llm_models')?.addEventListener('click', () => loadLLMModels().catch((error) => log(String(error))));

byId('provider_init')?.addEventListener('click', () => initProviders().catch((error) => providerLog(String(error))));
byId('provider_apply')?.addEventListener('click', () => applyManagedProviders().catch((error) => providerLog(String(error))));
byId('provider_export')?.addEventListener('click', () => exportReadiness().catch((error) => providerLog(String(error))));
byId('provider_copy_audit')?.addEventListener('click', () => copyAuditSummary().catch((error) => providerLog(String(error))));
byId('provider_open_dir')?.addEventListener('click', () => openReadinessDir().catch((error) => providerLog(String(error))));

byId('llm_endpoint')?.addEventListener('blur', () => {
  const endpoint = normalizeEndpoint(getValue('llm_endpoint'), 'http://127.0.0.1:11434');
  setInput('llm_endpoint', endpoint);
  loadLLMModels().catch((error) => log(String(error)));
});

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