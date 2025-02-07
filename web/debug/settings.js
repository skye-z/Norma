const api = 'http://127.0.0.1:8090';
const logEl = document.getElementById('log');

const byId = (id) => document.getElementById(id);
const log = (s) => { if (logEl) logEl.textContent = `[${new Date().toLocaleTimeString()}] ${s}\n` + logEl.textContent; };

function apply(cfg) {
  byId('llm_provider').value = cfg.llm?.provider || '';
  byId('llm_endpoint').value = cfg.llm?.endpoint || '';
  byId('llm_model').value = cfg.llm?.model || '';
  byId('llm_timeout').value = cfg.llm?.request_timeout_ms || 45000;

  byId('asr_provider').value = cfg.asr?.provider || '';
  byId('asr_endpoint').value = cfg.asr?.endpoint || '';

  byId('tts_provider').value = cfg.tts?.provider || '';
  byId('tts_endpoint').value = cfg.tts?.endpoint || '';

  byId('rag_topk').value = cfg.rag?.top_k || 6;
  byId('stm_budget').value = cfg.memory?.stm_token_budget || 2048;

  byId('perf_auto').checked = !!cfg.perf?.auto_degrade;
  byId('perf_first').value = cfg.perf?.llm_first_token_threshold_ms || 1200;
  byId('perf_topk').value = cfg.perf?.degraded_rag_top_k || 4;
}

function collect(base) {
  const cfg = structuredClone(base);
  cfg.llm.provider = byId('llm_provider').value.trim();
  cfg.llm.endpoint = byId('llm_endpoint').value.trim();
  cfg.llm.model = byId('llm_model').value.trim();
  cfg.llm.request_timeout_ms = Number(byId('llm_timeout').value || 45000);

  cfg.asr.provider = byId('asr_provider').value.trim();
  cfg.asr.endpoint = byId('asr_endpoint').value.trim();

  cfg.tts.provider = byId('tts_provider').value.trim();
  cfg.tts.endpoint = byId('tts_endpoint').value.trim();

  cfg.rag.top_k = Number(byId('rag_topk').value || 6);
  cfg.memory.stm_token_budget = Number(byId('stm_budget').value || 2048);

  cfg.perf.auto_degrade = !!byId('perf_auto').checked;
  cfg.perf.llm_first_token_threshold_ms = Number(byId('perf_first').value || 1200);
  cfg.perf.degraded_rag_top_k = Number(byId('perf_topk').value || 4);
  return cfg;
}

let current = null;

async function load() {
  const resp = await fetch(`${api}/settings/runtime`);
  if (!resp.ok) throw new Error(`load failed: ${resp.status}`);
  current = await resp.json();
  apply(current);
  log('已读取配置');
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
    throw new Error(`save failed: ${resp.status} ${err}`);
  }
  log('保存成功，已触发热更新事件');
  await load();
}

byId('load').addEventListener('click', () => load().catch((e) => log(String(e))));
byId('save').addEventListener('click', () => save().catch((e) => log(String(e))));

load().catch((e) => log(String(e)));