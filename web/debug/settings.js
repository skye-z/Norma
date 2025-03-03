const api = 'http://127.0.0.1:8090';
const logEl = document.getElementById('log');
const providerLogEl = document.getElementById('provider_log');
const providerSummaryEl = document.getElementById('provider_summary');
const providerCatalogEl = document.getElementById('provider_catalog');

const byId = (id) => document.getElementById(id);
const csv = (value) => String(value || '').split(',').map((item) => item.trim()).filter(Boolean);
const log = (s) => {
  if (logEl) {
    logEl.textContent = `[${new Date().toLocaleTimeString()}] ${s}\n` + logEl.textContent;
  }
};
const providerLog = (s) => {
  if (providerLogEl) {
    providerLogEl.textContent = `[${new Date().toLocaleTimeString()}] ${s}\n` + providerLogEl.textContent;
  }
};

function apply(cfg) {
  byId('llm_provider').value = cfg.llm?.provider || '';
  byId('llm_endpoint').value = cfg.llm?.endpoint || '';
  byId('llm_model').value = cfg.llm?.model || '';
  byId('llm_timeout').value = cfg.llm?.request_timeout_ms || 45000;

  byId('asr_provider').value = cfg.asr?.provider || '';
  byId('asr_binary_path').value = cfg.asr?.binary_path || '';
  byId('asr_model_path').value = cfg.asr?.model_path || '';

  byId('tts_provider').value = cfg.tts?.provider || '';
  byId('tts_voice').value = cfg.tts?.voice || '';
  byId('tts_binary_path').value = cfg.tts?.binary_path || '';
  byId('tts_model_path').value = cfg.tts?.model_path || '';
  byId('tts_sentence_first').checked = !!cfg.tts?.sentence_first_enabled;

  byId('audio_wake_mode').value = cfg.audio?.wake_mode || 'vad';
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
  byId('perf_audio_chunk').value = cfg.perf?.degraded_audio_chunk_size || 4096;
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
  cfg.llm.endpoint = byId('llm_endpoint').value.trim();
  cfg.llm.model = byId('llm_model').value.trim();
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
  cfg.perf.degraded_audio_chunk_size = Number(byId('perf_audio_chunk').value || 4096);
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

function renderProviderSummary(status) {
  providerSummaryEl.innerHTML = '';
  const lines = [
    ['根目录', status.root || '-'],
    ['当前 Wake', status.stack?.wake || '-'],
    ['当前 ASR', status.stack?.asr || '-'],
    ['当前 TTS', status.stack?.tts || '-'],
    ['当前 Embedding', status.stack?.embedding || '-'],
    ['Manifest 记录数', String(status.manifest?.providers?.length || 0)]
  ];
  for (const [label, value] of lines) {
    const row = document.createElement('div');
    row.className = 'stat-line';
    row.innerHTML = `<span>${label}</span><strong>${value}</strong>`;
    providerSummaryEl.appendChild(row);
  }
  for (const [kind, files] of Object.entries(status.providers || {})) {
    const row = document.createElement('div');
    row.className = 'stat-line';
    row.innerHTML = `<span>${kind}</span><span>${(files || []).join(', ') || 'empty'}</span>`;
    providerSummaryEl.appendChild(row);
  }
}

function renderProviderCatalog(catalog) {
  providerCatalogEl.innerHTML = '';
  for (const item of catalog.items || []) {
    const card = document.createElement('div');
    card.className = 'metric-tile';
    const note = item.notes ? `<div class="metric-foot">${item.notes}</div>` : '';
    card.innerHTML = `
      <div class="metric-label">${item.kind.toUpperCase()} / ${item.version}</div>
      <div class="metric-value" style="font-size:18px;">${item.name}</div>
      <div class="metric-foot">${item.id}</div>
      ${note}
      <div class="hero-actions" style="margin-top:12px;">
        <button data-install="${item.id}" class="primary">安装</button>
        <button data-uninstall="${item.id}" class="danger">卸载</button>
      </div>
    `;
    providerCatalogEl.appendChild(card);
  }
  providerCatalogEl.querySelectorAll('[data-install]').forEach((btn) => {
    btn.addEventListener('click', async () => installProvider(btn.dataset.install));
  });
  providerCatalogEl.querySelectorAll('[data-uninstall]').forEach((btn) => {
    btn.addEventListener('click', async () => uninstallProvider(btn.dataset.uninstall));
  });
}

let current = null;

async function load() {
  const resp = await fetch(`${api}/settings/runtime`);
  if (!resp.ok) throw new Error(`读取配置失败: ${resp.status}`);
  current = await resp.json();
  apply(current);
  log('已读取当前运行时配置。');
}

async function loadProviderStatus() {
  const resp = await fetch(`${api}/providers/status`);
  if (!resp.ok) throw new Error(`读取 Provider 状态失败: ${resp.status}`);
  const data = await resp.json();
  renderProviderSummary(data);
  providerLog('已刷新 Provider 状态。');
}

async function loadProviderCatalog() {
  const resp = await fetch(`${api}/providers/catalog`);
  if (!resp.ok) throw new Error(`读取 Provider 清单失败: ${resp.status}`);
  const data = await resp.json();
  renderProviderCatalog(data);
}

async function save() {
  if (!current) return;
  const body = collect(current);
  const resp = await fetch(`${api}/settings/runtime`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  if (!resp.ok) {
    const err = await resp.text();
    throw new Error(`保存配置失败: ${resp.status} ${err}`);
  }
  log('配置保存成功，已广播热更新事件。');
  await load();
  await loadProviderStatus();
}

async function initProviders() {
  const resp = await fetch(`${api}/providers/init`, { method: 'POST' });
  if (!resp.ok) throw new Error(`初始化 Provider 目录失败: ${resp.status}`);
  providerLog('Provider 目录初始化完成。');
  await loadProviderStatus();
}

async function installProvider(id) {
  providerLog(`开始安装 ${id} ...`);
  const resp = await fetch(`${api}/providers/install`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id })
  });
  const data = await resp.json();
  if (!resp.ok) throw new Error(data?.error?.message || `安装失败: ${resp.status}`);
  providerLog(String(data.output || `${id} 安装成功`));
  await loadProviderStatus();
}

async function uninstallProvider(id) {
  providerLog(`开始卸载 ${id} ...`);
  const resp = await fetch(`${api}/providers/uninstall`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id })
  });
  const data = await resp.json();
  if (!resp.ok) throw new Error(data?.error?.message || `卸载失败: ${resp.status}`);
  providerLog(String(data.output || `${id} 卸载成功`));
  await loadProviderStatus();
}

byId('load').addEventListener('click', () => Promise.all([load(), loadProviderStatus(), loadProviderCatalog()]).catch((e) => log(String(e))));
byId('save').addEventListener('click', () => save().catch((e) => log(String(e))));
byId('provider_init').addEventListener('click', () => initProviders().catch((e) => providerLog(String(e))));

Promise.all([load(), loadProviderStatus(), loadProviderCatalog()]).catch((e) => {
  log(String(e));
  providerLog(String(e));
});
