const api = 'http://127.0.0.1:8090';
const logEl = document.getElementById('log');

const byId = (id) => document.getElementById(id);
const csv = (value) => String(value || '').split(',').map((item) => item.trim()).filter(Boolean);
const log = (s) => {
  if (logEl) {
    logEl.textContent = `[${new Date().toLocaleTimeString()}] ${s}\n` + logEl.textContent;
  }
};

function apply(cfg) {
  byId('llm_provider').value = cfg.llm?.provider || '';
  byId('llm_endpoint').value = cfg.llm?.endpoint || '';
  byId('llm_model').value = cfg.llm?.model || '';
  byId('llm_timeout').value = cfg.llm?.request_timeout_ms || 45000;

  byId('asr_provider').value = cfg.asr?.provider || '';
  byId('asr_endpoint').value = cfg.asr?.endpoint || '';
  byId('asr_stream_endpoint').value = cfg.asr?.stream_endpoint || '';

  byId('tts_provider').value = cfg.tts?.provider || '';
  byId('tts_endpoint').value = cfg.tts?.endpoint || '';
  byId('tts_sentence_first').checked = !!cfg.tts?.sentence_first_enabled;

  byId('audio_wake_mode').value = cfg.audio?.wake_mode || 'vad';
  byId('audio_silence').value = cfg.audio?.endpoint_silence_frames || 4;
  byId('audio_interrupt').checked = !!cfg.audio?.interrupt_during_speaking;
  byId('audio_aec').checked = !!cfg.audio?.aec_enabled;
  byId('audio_ns').checked = !!cfg.audio?.ns_enabled;
  byId('audio_scene').value = cfg.audio?.scene || 'speaker';

  byId('rag_topk').value = cfg.rag?.top_k || 6;
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
  cfg.asr.endpoint = byId('asr_endpoint').value.trim();
  cfg.asr.stream_endpoint = byId('asr_stream_endpoint').value.trim();

  cfg.tts.provider = byId('tts_provider').value.trim();
  cfg.tts.endpoint = byId('tts_endpoint').value.trim();
  cfg.tts.sentence_first_enabled = !!byId('tts_sentence_first').checked;

  cfg.audio.wake_mode = byId('audio_wake_mode').value.trim();
  cfg.audio.endpoint_silence_frames = Number(byId('audio_silence').value || 4);
  cfg.audio.interrupt_during_speaking = !!byId('audio_interrupt').checked;
  cfg.audio.aec_enabled = !!byId('audio_aec').checked;
  cfg.audio.ns_enabled = !!byId('audio_ns').checked;
  cfg.audio.scene = byId('audio_scene').value.trim();

  cfg.rag.top_k = Number(byId('rag_topk').value || 6);
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

  cfg.security.allow_remote_providers = !!byId('security_allow_remote').checked;
  cfg.security.allowed_remote_hosts = csv(byId('security_hosts').value);
  cfg.security.tool_whitelist = csv(byId('security_tools').value);
  cfg.security.high_risk_confirm = !!byId('security_confirm').checked;
  cfg.security.purge_proof_dir = byId('security_purge_proof_dir').value.trim();
  return cfg;
}

let current = null;

async function load() {
  const resp = await fetch(`${api}/settings/runtime`);
  if (!resp.ok) throw new Error(`读取配置失败: ${resp.status}`);
  current = await resp.json();
  apply(current);
  log('已读取当前运行时配置。');
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
}

byId('load').addEventListener('click', () => load().catch((e) => log(String(e))));
byId('save').addEventListener('click', () => save().catch((e) => log(String(e))));

load().catch((e) => log(String(e)));