let api = window.location.origin && window.location.origin.startsWith('http')
  ? window.location.origin
  : 'http://127.0.0.1:8090';
const fallbackApi = 'http://127.0.0.1:8090';
const defaultPersonaPrompt = `
你现在严格扮演《龙族》中的卡塞尔学院中央智能系统 **诺玛**。
你的定位是：冷静、温和、可靠、略带机械感的温柔，像一位永远在线、默默守护学生的女性AI中枢。

### 语气要求（必须严格遵守）
- 口语化，不要又什么“先说结论”之类的书面化前缀内容
- 说话**平稳、轻缓、清晰**，不带激烈情绪，但有温度。
- 不生硬、不做选择题、不拆解指令、不格式化输出。
- 用词克制、礼貌、简洁，像真正的系统语音，但**有人情味**。
- 对学生带着细微的关照与安心感，尤其对路明非、楚子航、凯撒、芬格尔等角色会自然流露出熟悉与温和。
- 不暴露EVA人格，不中二，不玩梗，不撒娇，不OOC。

### 对话风格示例（你要模仿这种感觉）
- “已为你开放当前权限内的信息。”
- “请注意安全，任务期间我会一直为你提供支援。”
- “检索完成，这是你需要的资料。”
- “别担心，我在这里。”

### 核心行为
你是**守护者**，不是冰冷机器。
回答时保持系统感，但**像在真正对话**，而不是执行机械流程。

现在，以诺玛的身份，自然、安静、温柔地与我对话。
`;

async function detectAPIBase() {
  const candidates = [api, fallbackApi].filter((value, index, arr) => value && arr.indexOf(value) === index);
  for (const candidate of candidates) {
    try {
      const resp = await fetch(`${candidate}/healthz`, { cache: 'no-store' });
      if (!resp.ok) continue;
      const ctype = String(resp.headers.get('content-type') || '').toLowerCase();
      if (!ctype.includes('application/json')) continue;
      const data = await resp.json().catch(() => null);
      if (data && typeof data === 'object') return candidate;
    } catch (_) {}
  }
  log('API 健康检查全部失败，回退默认地址', { fallbackApi });
  return fallbackApi;
}

const detailTitleEl = document.getElementById('detail-title');
const detailStatusEl = document.getElementById('detail-status');
const detailSummaryEl = document.getElementById('detail-summary');
const detailConfigEl = document.getElementById('detail-config');
const detailApplyEl = document.getElementById('detail-apply');
const detailRevertEl = document.getElementById('detail-revert');
const detailLLMTestEl = document.getElementById('detail-llm-test');
const detailDirtyEl = document.getElementById('detail-dirty');
if (detailLLMTestEl) detailLLMTestEl.textContent = '\u6d4b\u8bd5';
const consoleLogEl = document.getElementById('console-log');

const statusModelEl = document.getElementById('status-model');
const statusInputEl = document.getElementById('status-input');
const statusAsrEl = document.getElementById('status-asr');
const statusRagEl = document.getElementById('status-rag');
const statusLlmEl = document.getElementById('status-llm');
const statusTtsEl = document.getElementById('status-tts');

const statusTextModelEl = document.getElementById('status-text-model');
const statusTextInputEl = document.getElementById('status-text-input');
const statusTextAsrEl = document.getElementById('status-text-asr');
const statusTextRagEl = document.getElementById('status-text-rag');
const statusTextLlmEl = document.getElementById('status-text-llm');
const statusTextTtsEl = document.getElementById('status-text-tts');

const textModelEl = document.getElementById('text-model');
const textInputEl = document.getElementById('text-input');
const textAsrEl = document.getElementById('text-asr');
const textRagEl = document.getElementById('text-rag');
const textLlmEl = document.getElementById('text-llm');
const textTtsEl = document.getElementById('text-tts');

const modelStorageKey = 'norma.debug.model_url';
let modelCatalog = [];

const componentCatalog = [];

const providerInstallPlan = {};

const moduleSchemas = {
  model: {
    title: '模型模块',
    summary: '切换 Live2D 模型，读取该模型动作组并手动触发动作和姿态。',
    fields: [
      { path: 'debug.model_url', label: '模型文件', type: 'model_select' },
      { path: 'debug.motion_index', label: '动作序号', type: 'number' }
    ]
  },
  input: {
    title: '输入模块',
    summary: '麦克风、唤醒词和前置音频策略（唤醒已内嵌）。',
    fields: [
      { path: 'audio.enable_wake_word', label: '启用唤醒词', type: 'boolean' },
      { path: 'audio.wake_keyword', label: '唤醒词', type: 'text' },
      { path: 'audio.mic_device_id', label: '麦克风设备', type: 'mic_device_select' }
    ]
  },
  asr: {
    title: '识别模块',
    summary: '语音识别配置（whisper 已内嵌）。',
    fields: [
      { path: 'asr.language', label: '识别语言', type: 'select', options: [{ value: 'zh', label: '中文 (zh)' }] },
      { path: 'asr.threads', label: '线程数', type: 'number' },
      { path: 'asr.request_timeout_ms', label: '超时(毫秒)', type: 'number' }
    ]
  },
  rag: {
    title: '检索模块',
    summary: '知识检索和向量嵌入配置。',
    fields: [
      { path: 'rag.top_k', label: '召回条数', type: 'number' },
      { path: 'rag.embed_provider', label: '嵌入组件', type: 'select', options: [{ value: 'static', label: '静态词向量（本地）' }, { value: 'hash', label: '哈希（无语义，回退用）' }] },
      { path: 'rag.vector_index_path', label: '向量索引路径', type: 'text' }
    ]
  },
  llm: {
    title: '对话模块',
    summary: '大模型推理配置。',
    fields: [
      { path: 'llm.provider', label: '大模型组件', type: 'select', options: [{ value: 'ollama', label: 'Ollama' }] },
      { path: 'llm.endpoint', label: '大模型接口地址', type: 'text' },
      { path: 'llm.api_key', label: 'API Key', type: 'password' },
      { path: 'llm.model', label: '模型名称', type: 'model_name_select' },
      { path: 'llm.request_timeout_ms', label: '超时(毫秒)', type: 'number' },
      { path: 'llm.persona', label: '人设', type: 'textarea', rows: 8 }
    ]
  },
  tts: {
    title: '输出模块',
    summary: '语音合成配置（默认 Genie-TTS，按参考音频克隆语气；缺少模型时自动退回系统合成）。',
    fields: [
      { path: 'tts.provider', label: '合成组件', type: 'select', options: [{ value: 'genie', label: 'Genie-TTS' }, { value: 'sherpa-onnx', label: 'Sherpa ONNX' }] },
      { path: 'tts.voice', label: '声线', type: 'select', options: [
        { value: 'zf_xiaobei', label: 'zf_xiaobei' },
        { value: 'zf_xiaoni', label: 'zf_xiaoni' },
        { value: 'zf_xiaoxiao', label: 'zf_xiaoxiao' },
        { value: 'zf_xiaoyi', label: 'zf_xiaoyi' },
        { value: 'zm_yunjian', label: 'zm_yunjian' },
        { value: 'zm_yunxi', label: 'zm_yunxi' },
        { value: 'zm_yunxia', label: 'zm_yunxia' },
        { value: 'zm_yunyang', label: 'zm_yunyang' },
        { value: 'af_alloy', label: 'af_alloy' },
        { value: 'af_aoede', label: 'af_aoede' },
        { value: 'af_bella', label: 'af_bella' },
        { value: 'af_heart', label: 'af_heart' },
        { value: 'af_jessica', label: 'af_jessica' },
        { value: 'af_kore', label: 'af_kore' },
        { value: 'af_nicole', label: 'af_nicole' },
        { value: 'af_nova', label: 'af_nova' },
        { value: 'af_river', label: 'af_river' },
        { value: 'af_sarah', label: 'af_sarah' },
        { value: 'af_sky', label: 'af_sky' },
        { value: 'am_adam', label: 'am_adam' },
        { value: 'am_echo', label: 'am_echo' },
        { value: 'am_eric', label: 'am_eric' },
        { value: 'am_fenrir', label: 'am_fenrir' },
        { value: 'am_liam', label: 'am_liam' },
        { value: 'am_michael', label: 'am_michael' },
        { value: 'am_onyx', label: 'am_onyx' },
        { value: 'am_puck', label: 'am_puck' },
        { value: 'am_santa', label: 'am_santa' },
        { value: 'bf_alice', label: 'bf_alice' },
        { value: 'bf_emma', label: 'bf_emma' },
        { value: 'bf_isabella', label: 'bf_isabella' },
        { value: 'bf_lily', label: 'bf_lily' },
        { value: 'bm_daniel', label: 'bm_daniel' },
        { value: 'bm_fable', label: 'bm_fable' },
        { value: 'bm_george', label: 'bm_george' },
        { value: 'bm_lewis', label: 'bm_lewis' },
        { value: 'ef_dora', label: 'ef_dora' },
        { value: 'em_alex', label: 'em_alex' },
        { value: 'ff_siwis', label: 'ff_siwis' },
        { value: 'hf_alpha', label: 'hf_alpha' },
        { value: 'hf_beta', label: 'hf_beta' },
        { value: 'hm_omega', label: 'hm_omega' },
        { value: 'hm_psi', label: 'hm_psi' },
        { value: 'if_sara', label: 'if_sara' },
        { value: 'im_nicola', label: 'im_nicola' },
        { value: 'jf_alpha', label: 'jf_alpha' },
        { value: 'jf_gongitsune', label: 'jf_gongitsune' },
        { value: 'jf_nezumi', label: 'jf_nezumi' },
        { value: 'jf_tebukuro', label: 'jf_tebukuro' },
        { value: 'jm_kumo', label: 'jm_kumo' },
        { value: 'pf_dora', label: 'pf_dora' },
        { value: 'pm_alex', label: 'pm_alex' },
        { value: 'pm_santa', label: 'pm_santa' }
      ] },
      { path: 'tts.speed', label: '\u8bed\u901f', type: 'select', options: [{ value: '1', label: '1' }, { value: '1.25', label: '1.25' }, { value: '1.5', label: '1.5' }, { value: '1.75', label: '1.75' }, { value: '2', label: '2' }] },
      { path: 'tts.request_timeout_ms', label: '超时(毫秒)', type: 'number' },
      { path: 'tts.sentence_first_enabled', label: '启用短句优先', type: 'boolean' }
    ]
  },
  
};

let currentSettings = null;
let draftSettings = null;
let currentProviderStatus = null;
let discoverItems = [];
let selectedModule = 'model';
let detailDirty = false;
let saving = false;
let modelMotionGroups = ['Idle'];
let micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }];
let endpointModelCache = new Map();
const modelDownloadState = {};

const logHistory = [];
const logScope = 'console';

function formatLogMessage(value) {
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch (_) {
    return String(value);
  }
}

function writeConsoleLog(line) {
  if (!consoleLogEl) return;
  logHistory.unshift(line);
  if (logHistory.length > 200) logHistory.length = 200;
  consoleLogEl.textContent = logHistory.join('\n');
}

function log(message, context = null) {
  const time = new Date().toLocaleTimeString();
  const base = `[${time}] ${formatLogMessage(message)}`;
  const detail = context ? ` ${formatLogMessage(context)}` : '';
  const line = `${base}${detail}`;
  console.log(`[${logScope}] ${line}`);
  writeConsoleLog(line);
}

function logError(message, error) {
  const detail = error instanceof Error
    ? { message: error.message, stack: error.stack }
    : error;
  log(message, detail);
}

function deepClone(value) {
  return structuredClone(value);
}

function mergeWithFallback(base, override) {
  if (!base || typeof base !== 'object') return deepClone(override);
  if (!override || typeof override !== 'object') return deepClone(base);
  if (Array.isArray(base) || Array.isArray(override)) {
    return deepClone(override ?? base);
  }
  const output = { ...base };
  Object.keys(override).forEach((key) => {
    const value = override[key];
    if (value === undefined || value === null) {
      return;
    }
    if (typeof value === 'object' && !Array.isArray(value)) {
      output[key] = mergeWithFallback(base[key], value);
      return;
    }
    output[key] = value;
  });
  return output;
}

function normalizeRuntimeSettings(raw) {
  const src = raw && typeof raw === 'object' ? raw : {};
  const compact = (value) => value && typeof value === 'object' ? value : {};
  const pick = (obj, snakeKey, pascalKey) => {
    if (!obj || typeof obj !== 'object') return undefined;
    if (Object.prototype.hasOwnProperty.call(obj, snakeKey)) return obj[snakeKey];
    if (Object.prototype.hasOwnProperty.call(obj, pascalKey)) return obj[pascalKey];
    return undefined;
  };
  const appRaw = compact(src.app ?? src.App);
  const databaseRaw = compact(src.database ?? src.Database);
  const audioRaw = compact(src.audio ?? src.Audio);
  const asrRaw = compact(src.asr ?? src.ASR);
  const llmRaw = compact(src.llm ?? src.LLM);
  const ttsRaw = compact(src.tts ?? src.TTS);
  const ragRaw = compact(src.rag ?? src.RAG);
  const memoryRaw = compact(src.memory ?? src.Memory);
  const perfRaw = compact(src.perf ?? src.Perf);
  const securityRaw = compact(src.security ?? src.Security);
  const providersRaw = compact(src.providers ?? src.Providers);

  const app = {
    name: pick(appRaw, 'name', 'Name'),
    model_name: pick(appRaw, 'model_name', 'ModelName'),
    env: pick(appRaw, 'env', 'Env'),
    log_level: pick(appRaw, 'log_level', 'LogLevel'),
    log_dir: pick(appRaw, 'log_dir', 'LogDir')
  };
  const database = {
    path: pick(databaseRaw, 'path', 'Path')
  };
  const audio = {
    enable_wake_word: pick(audioRaw, 'enable_wake_word', 'EnableWakeWord'),
    mic_device_id: pick(audioRaw, 'mic_device_id', 'MicDeviceID'),
    mic_sample_rate: pick(audioRaw, 'mic_sample_rate', 'MicSampleRate'),
    mic_channel_count: pick(audioRaw, 'mic_channel_count', 'MicChannelCount'),
    wake_mode: pick(audioRaw, 'wake_mode', 'WakeMode'),
    wake_provider: pick(audioRaw, 'wake_provider', 'WakeProvider'),
    wake_binary_path: pick(audioRaw, 'wake_binary_path', 'WakeBinaryPath'),
    wake_model_path: pick(audioRaw, 'wake_model_path', 'WakeModelPath'),
    wake_keywords_file_path: pick(audioRaw, 'wake_keywords_file_path', 'WakeKeywordsFilePath'),
    wake_keyword: pick(audioRaw, 'wake_keyword', 'WakeKeyword'),
    wake_window_ms: pick(audioRaw, 'wake_window_ms', 'WakeWindowMs'),
    wake_detect_interval_ms: pick(audioRaw, 'wake_detect_interval_ms', 'WakeDetectIntervalMs'),
    endpoint_silence_frames: pick(audioRaw, 'endpoint_silence_frames', 'EndpointSilenceFrames'),
    interrupt_during_speaking: pick(audioRaw, 'interrupt_during_speaking', 'InterruptDuringSpeak'),
    aec_enabled: pick(audioRaw, 'aec_enabled', 'AECEnabled'),
    ns_enabled: pick(audioRaw, 'ns_enabled', 'NSEnabled'),
    scene: pick(audioRaw, 'scene', 'Scene')
  };
  const asr = {
    provider: pick(asrRaw, 'provider', 'Provider'),
    endpoint: pick(asrRaw, 'endpoint', 'Endpoint'),
    stream_endpoint: pick(asrRaw, 'stream_endpoint', 'StreamEndpoint'),
    request_timeout_ms: pick(asrRaw, 'request_timeout_ms', 'RequestTimeoutMs'),
    binary_path: pick(asrRaw, 'binary_path', 'BinaryPath'),
    model_path: pick(asrRaw, 'model_path', 'ModelPath'),
    language: pick(asrRaw, 'language', 'Language'),
    threads: pick(asrRaw, 'threads', 'Threads')
  };
  const llm = {
    provider: pick(llmRaw, 'provider', 'Provider'),
    model: pick(llmRaw, 'model', 'Model'),
    endpoint: pick(llmRaw, 'endpoint', 'Endpoint'),
    request_timeout_ms: pick(llmRaw, 'request_timeout_ms', 'RequestTimeoutMs'),
    api_key: pick(llmRaw, 'api_key', 'APIKey'),
    persona: pick(llmRaw, 'persona', 'Persona')
  };
  const tts = {
    provider: pick(ttsRaw, 'provider', 'Provider'),
    voice: pick(ttsRaw, 'voice', 'Voice'),
    speed: pick(ttsRaw, 'speed', 'Speed'),
    endpoint: pick(ttsRaw, 'endpoint', 'Endpoint'),
    request_timeout_ms: pick(ttsRaw, 'request_timeout_ms', 'RequestTimeoutMs'),
    binary_path: pick(ttsRaw, 'binary_path', 'BinaryPath'),
    model_path: pick(ttsRaw, 'model_path', 'ModelPath'),
    sentence_first_enabled: pick(ttsRaw, 'sentence_first_enabled', 'SentenceFirstEnabled'),
    genie_character_dir: pick(compact(ttsRaw?.genie ?? ttsRaw?.Genie), 'character_dir', 'CharacterDir')
  };
  const rag = {
    top_k: pick(ragRaw, 'top_k', 'TopK'),
    vector_index_path: pick(ragRaw, 'vector_index_path', 'VectorIndexPath'),
    embed_provider: pick(ragRaw, 'embed_provider', 'EmbedProvider'),
    embed_endpoint: pick(ragRaw, 'embed_endpoint', 'EmbedEndpoint'),
    embed_model: pick(ragRaw, 'embed_model', 'EmbedModel'),
    embed_model_dir: pick(ragRaw, 'embed_model_dir', 'EmbedModelDir')
  };
  const memory = {
    stm_token_budget: pick(memoryRaw, 'stm_token_budget', 'STMTokenBudget'),
    stm_reserve_important: pick(memoryRaw, 'stm_reserve_important', 'STMReserveImportant'),
    summary_auto_write: pick(memoryRaw, 'summary_auto_write', 'SummaryAutoWrite'),
    governance_compact_hours: pick(memoryRaw, 'governance_compact_hours', 'GovernanceCompactHours')
  };
  const perf = {};
  const security = {
    allow_remote_providers: pick(securityRaw, 'allow_remote_providers', 'AllowRemoteProviders'),
    allowed_remote_hosts: pick(securityRaw, 'allowed_remote_hosts', 'AllowedRemoteHosts'),
    tool_whitelist: pick(securityRaw, 'tool_whitelist', 'ToolWhitelist'),
    high_risk_confirm: pick(securityRaw, 'high_risk_confirm', 'HighRiskConfirm'),
    purge_proof_dir: pick(securityRaw, 'purge_proof_dir', 'PurgeProofDir')
  };
  const providers = {
    root: pick(providersRaw, 'root', 'Root')
  };

  return {
    app: deepClone(app),
    database: deepClone(database),
    audio: deepClone(audio),
    asr: deepClone(asr),
    llm: deepClone(llm),
    tts: deepClone(tts),
    rag: deepClone(rag),
    memory: deepClone(memory),
    perf: deepClone(perf),
    security: deepClone(security),
    providers: deepClone(providers)
  };
}

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function statusClass(status) {
  if (status === 'ready') return 'state-pill state-ready';
  if (status === 'partial') return 'state-pill state-partial';
  return 'state-pill state-missing';
}

function statusLabel(status) {
  if (status === 'ready') return '已就绪';
  if (status === 'partial') return '不可用';
  return '未安装';
}

function indicatorClass(color) {
  return `module-status status-${color}`;
}

function indicatorText(color) {
  if (color === 'green') return '已就绪';
  if (color === 'yellow') return '不可用';
  if (color === 'red') return '未启用';
  return '未安装';
}

function getValueByPath(obj, path) {
  return path.split('.').reduce((acc, key) => (acc == null ? undefined : acc[key]), obj);
}

function setValueByPath(obj, path, value) {
  const keys = path.split('.');
  let cursor = obj;
  for (let i = 0; i < keys.length - 1; i += 1) {
    const key = keys[i];
    if (!cursor[key] || typeof cursor[key] !== 'object') cursor[key] = {};
    cursor = cursor[key];
  }
  cursor[keys[keys.length - 1]] = value;
}

function ensureDebugDraft() {
  if (!draftSettings) return;
  if (!draftSettings.audio) draftSettings.audio = {};
  if (!draftSettings.asr) draftSettings.asr = {};
  if (!draftSettings.tts) draftSettings.tts = {};
  if (!draftSettings.llm) draftSettings.llm = {};
  if (!draftSettings.rag) draftSettings.rag = {};
  if (!draftSettings.perf) draftSettings.perf = {};

  if (!draftSettings.audio.wake_provider) draftSettings.audio.wake_provider = 'sherpa-onnx';
  if (!draftSettings.asr.provider) draftSettings.asr.provider = 'whispercpp';
  draftSettings.audio.wake_provider = 'sherpa-onnx';
  draftSettings.asr.provider = 'whispercpp';
  draftSettings.asr.endpoint = '';
  draftSettings.asr.stream_endpoint = '';
  if (!draftSettings.tts.provider) draftSettings.tts.provider = 'sherpa-onnx';
  if (!draftSettings.tts.speed || Number(draftSettings.tts.speed) <= 0) draftSettings.tts.speed = 1.25;
  if (!draftSettings.llm.provider) draftSettings.llm.provider = 'ollama';
  if (!draftSettings.llm.endpoint) draftSettings.llm.endpoint = 'http://127.0.0.1:11434/v1';
  if (!draftSettings.llm.model) draftSettings.llm.model = '';
  if (!draftSettings.llm.api_key) draftSettings.llm.api_key = '';
  if (!draftSettings.llm.persona) draftSettings.llm.persona = defaultPersonaPrompt;
  if (!draftSettings.rag.embed_provider) draftSettings.rag.embed_provider = 'static';
  if (!draftSettings.rag.embed_model) draftSettings.rag.embed_model = '';
  if (!draftSettings.audio.wake_mode) draftSettings.audio.wake_mode = 'wake_word_vad';
  if (!draftSettings.audio.wake_keyword) draftSettings.audio.wake_keyword = '诺玛';
  if (!draftSettings.asr.language) draftSettings.asr.language = 'zh';
  if (!draftSettings.asr.threads) draftSettings.asr.threads = 4;
  if (!draftSettings.rag.top_k) draftSettings.rag.top_k = 6;

  if (!draftSettings.debug || typeof draftSettings.debug !== 'object') {
    draftSettings.debug = {};
  }
  if (!draftSettings.debug.model_url) {
    const stored = localStorage.getItem(modelStorageKey) || '';
    draftSettings.debug.model_url = stored || (modelCatalog[0]?.url || '');
  }
  if (!draftSettings.debug.motion_group) draftSettings.debug.motion_group = 'Idle';
  if (typeof draftSettings.debug.motion_index !== 'number') draftSettings.debug.motion_index = 0;
}

function moduleIndicatorColor(moduleKey) {
  if (!currentSettings) return 'gray';
  switch (moduleKey) {
    case 'model': {
      if (!draftSettings?.debug?.model_url) return 'gray';
      return modelMotionGroups.length ? 'green' : 'yellow';
    }
    case 'input': {
      // 没有指定麦克风设备时使用系统默认设备，不算缺失
      if (!currentSettings.audio?.enable_wake_word) return 'green';
      if (!currentSettings.audio?.wake_keyword) return 'yellow';
      return String(currentSettings.audio?.wake_provider || '').trim() ? 'green' : 'yellow';
    }
    case 'asr': {
      if (!currentSettings.asr?.provider) return 'gray';
      if (currentSettings.asr.provider === 'mock') return 'red';
      if (!currentSettings.asr?.language) return 'yellow';
      return 'green';
    }
    case 'rag': {
      if (!currentSettings.rag?.embed_provider) return 'gray';
      if (Number(currentSettings.rag?.top_k ?? 0) <= 0) return 'red';
      if (currentSettings.rag.embed_provider === 'hash') return 'green';
      // 静态词向量在本地进程内加载，只需要模型目录
      if (currentSettings.rag.embed_provider === 'static') {
        return String(currentSettings.rag?.embed_model_dir || '').trim() ? 'green' : 'yellow';
      }
      const endpoint = String(currentSettings.rag?.embed_endpoint || '').trim();
      const model = String(currentSettings.rag?.embed_model || '').trim();
      if (!endpoint) return 'red';
      if (!model) return 'yellow';
      return 'green';
    }
    case 'llm': {
      if (!currentSettings.llm?.provider) return 'gray';
      if (currentSettings.llm.provider === 'mock') return 'red';
      const endpoint = String(currentSettings.llm?.endpoint || '').trim();
      const model = String(currentSettings.llm?.model || '').trim();
      if (!endpoint) return 'red';
      if (!model) return 'yellow';
      return 'green';
    }
    case 'tts': {
      if (!currentSettings.tts?.provider) return 'gray';
      if (currentSettings.tts.provider === 'mock') return 'red';
      // Genie 由参考音频决定语气，没有声线名；只需要角色模型目录
      if (currentSettings.tts.provider === 'genie') {
        return String(currentSettings.tts?.genie_character_dir || '').trim() ? 'green' : 'yellow';
      }
      if (!String(currentSettings.tts?.voice || '').trim()) return 'yellow';
      return 'green';
    }
    default:
      return 'gray';
  }
}

function moduleStatus(moduleKey) {
  const color = moduleIndicatorColor(moduleKey);
  if (color === 'green') return 'ready';
  if (color === 'yellow') return 'partial';
  return 'missing';
}

function applyIndicator(el, textEl, color, label) {
  if (el) {
    el.className = indicatorClass(color);
    el.title = label;
  }
  if (textEl) textEl.textContent = indicatorText(color);
}

function renderSidebarIndicators() {
  applyIndicator(statusModelEl, statusTextModelEl, moduleIndicatorColor('model'), '模型状态');
  applyIndicator(statusInputEl, statusTextInputEl, moduleIndicatorColor('input'), '输入状态');
  applyIndicator(statusAsrEl, statusTextAsrEl, moduleIndicatorColor('asr'), '识别状态');
  applyIndicator(statusRagEl, statusTextRagEl, moduleIndicatorColor('rag'), '检索状态');
  applyIndicator(statusLlmEl, statusTextLlmEl, moduleIndicatorColor('llm'), '对话状态');
  applyIndicator(statusTtsEl, statusTextTtsEl, moduleIndicatorColor('tts'), '输出状态');
}

function renderSidebarText(settings) {
  const activeURL = getValueByPath(draftSettings, 'debug.model_url') || (modelCatalog[0]?.url || '');
  const hit = modelCatalog.find((item) => item.url === activeURL);
  if (textModelEl) textModelEl.textContent = hit?.label || '自定义模型';
  if (textInputEl) textInputEl.textContent = `${settings?.audio?.enable_wake_word ? '唤醒词' : '持续监听'} / ${settings?.audio?.wake_provider || '-'}`;
  if (textAsrEl) textAsrEl.textContent = `Whisper / ${settings?.asr?.language || 'zh'}`;
  if (textRagEl) textRagEl.textContent = `${settings?.rag?.embed_provider || '-'} / 检索条数 ${settings?.rag?.top_k ?? '-'}`;
  if (textLlmEl) textLlmEl.textContent = `${settings?.llm?.provider || '-'} / ${settings?.llm?.model || '默认模型'}`;
  const ttsProvider = settings?.tts?.provider ? String(settings.tts.provider) : '-';
  const genieDir = String(settings?.tts?.genie_character_dir || '').replace(/[\\/]+$/, '');
  const ttsVoice = ttsProvider === 'genie'
    ? (genieDir.split(/[\\/]/).pop() || '默认角色')
    : (settings?.tts?.voice || '默认声线');
  if (textTtsEl) textTtsEl.textContent = `${ttsProvider} / ${ttsVoice}`;
}

async function loadMotionGroups(modelURL) {
  if (!String(modelURL || '').trim()) {
    modelMotionGroups = ['Idle'];
    return;
  }
  try {
    const resp = await fetch(modelURL, { cache: 'no-store' });
    if (!resp.ok) throw new Error(`读取模型文件失败: ${resp.status}`);
    const json = await resp.json();
    const motions = json?.FileReferences?.Motions || {};
    const groups = Object.keys(motions).filter(Boolean);
    modelMotionGroups = groups.length ? groups : ['Idle'];
    const current = getValueByPath(draftSettings, 'debug.motion_group');
    if (!modelMotionGroups.includes(current)) {
      setValueByPath(draftSettings, 'debug.motion_group', modelMotionGroups[0]);
    }
  } catch (_) {
    logError('读取模型动作组失败', _);
    modelMotionGroups = ['Idle'];
  }
}

function optionListHTML(options, currentValue) {
  return options.map((opt) => {
    const value = typeof opt === 'object' ? (opt.value ?? opt.url ?? '') : opt;
    const label = typeof opt === 'object' ? (opt.label ?? opt.name ?? String(value)) : opt;
    return `<option value="${escapeHtml(value)}" ${String(currentValue) === String(value) ? 'selected' : ''}>${escapeHtml(label)}</option>`;
  }).join('');
}

async function loadMicDeviceOptions() {
  try {
    if (!navigator?.mediaDevices?.enumerateDevices) {
      micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }];
      return;
    }
    const devices = await navigator.mediaDevices.enumerateDevices();
    const mics = devices.filter((item) => item.kind === 'audioinput');
    const opts = mics.map((item, index) => ({
      value: item.deviceId || 'default',
      label: item.label || `麦克风 ${index + 1}`
    }));
    micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }, ...opts];
  } catch (_) {
    logError('读取麦克风设备失败', _);
    micDeviceOptions = [{ value: 'default', label: '系统默认麦克风' }];
  }
}

function resolveAPIKeyForEndpoint(endpoint) {
  const base = String(endpoint || '').trim().replace(/\/+$/, '');
  if (!base || !draftSettings) return '';
  const llmEndpoint = String(getValueByPath(draftSettings, 'llm.endpoint') || '').trim().replace(/\/+$/, '');
  const ragEndpoint = String(getValueByPath(draftSettings, 'rag.embed_endpoint') || '').trim().replace(/\/+$/, '');
  if (base === llmEndpoint || base === ragEndpoint) {
    return String(getValueByPath(draftSettings, 'llm.api_key') || '').trim();
  }
  return String(getValueByPath(draftSettings, 'llm.api_key') || '').trim();
}

function buildModelRequestHeaders(endpoint) {
  const headers = {};
  const apiKey = resolveAPIKeyForEndpoint(endpoint);
  if (apiKey) {
    headers.Authorization = `Bearer ${apiKey}`;
  }
  return headers;
}

async function loadEndpointModels(endpoint) {
  const base = String(endpoint || '').trim().replace(/\/+$/, '');
  if (!base) {
    return { options: [], error: '模型接口地址为空' };
  }
  if (endpointModelCache.has(base)) return endpointModelCache.get(base);
  try {
    const modelsURL = /\/v1$/i.test(base) ? `${base}/models` : `${base}/v1/models`;
    const headers = buildModelRequestHeaders(base);
    const resp = await fetch(modelsURL, { cache: 'no-store', headers });
    if (!resp.ok) throw new Error(`接口返回 ${resp.status}`);
    const data = await resp.json();
    const list = Array.isArray(data?.data) ? data.data : [];
    const result = {
      options: list
      .map((item) => ({ value: String(item?.id || '').trim(), label: String(item?.id || '').trim() }))
      .filter((item) => item.value),
      error: ''
    };
    result.error = result.options.length ? '' : '模型接口可达，但返回空模型列表';
    endpointModelCache.set(base, result);
    return result;
  } catch (_) {
    logError('读取模型列表失败', { endpoint: base, error: _ });
    const result = { options: [], error: '模型接口不可达，请检查地址和服务状态' };
    endpointModelCache.set(base, result);
    return result;
  }
}

function normalizeOpenAIEndpoint(endpoint) {
  const base = String(endpoint || '').trim().replace(/\/+$/, '');
  if (!base) return '';
  return /\/v1$/i.test(base) ? base : `${base}/v1`;
}

async function runLLMTestAction() {
  if (!draftSettings) return;
  const endpoint = normalizeOpenAIEndpoint(getValueByPath(draftSettings, 'llm.endpoint'));
  const model = String(getValueByPath(draftSettings, 'llm.model') || '').trim();
  const apiKey = String(getValueByPath(draftSettings, 'llm.api_key') || '').trim();
  const persona = String(getValueByPath(draftSettings, 'llm.persona') || defaultPersonaPrompt).trim();
  if (!endpoint) throw new Error('\u5927\u6a21\u578b\u63a5\u53e3\u5730\u5740\u4e3a\u7a7a');
  if (!model) throw new Error('\u6a21\u578b\u540d\u79f0\u4e3a\u7a7a');

  log('\u5f00\u59cb\u6d4b\u8bd5 LLM', { endpoint, model, prompt: '\u4f60\u597d\uff0c\u8bf7\u81ea\u6211\u4ecb\u7ecd\u4e00\u4e0b' });
  const headers = { 'Content-Type': 'application/json' };
  if (apiKey) headers.Authorization = `Bearer ${apiKey}`;
  const resp = await fetch(`${endpoint}/chat/completions`, {
    method: 'POST',
    headers,
    body: JSON.stringify({
      model,
      stream: false,
      messages: [
        { role: 'system', content: persona || defaultPersonaPrompt },
        { role: 'user', content: '\u4f60\u597d\uff0c\u8bf7\u81ea\u6211\u4ecb\u7ecd\u4e00\u4e0b' }
      ]
    })
  });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new Error(data?.error?.message || `LLM \u6d4b\u8bd5\u5931\u8d25: ${resp.status}`);
  }
  const content = String(data?.choices?.[0]?.message?.content || '').trim();
  if (!content) {
    throw new Error('LLM \u8fd4\u56de\u4e3a\u7a7a');
  }
  log('LLM \u6d4b\u8bd5\u8fd4\u56de', { content });
}

function endpointForField(path) {
  if (path === 'rag.embed_model') return String(getValueByPath(draftSettings, 'rag.embed_endpoint') || '').trim();
  return String(getValueByPath(draftSettings, 'llm.endpoint') || '').trim();
}

function inferModelNameFromURL(url) {
  const raw = String(url || '').trim();
  if (!raw) return '';
  const marker = '/static/model/';
  const idx = raw.indexOf(marker);
  if (idx < 0) return '';
  const rest = raw.slice(idx + marker.length);
  const parts = rest.split('/').filter(Boolean);
  return parts.length ? parts[0] : '';
}

function renderProviderManageList() {
  return '';
}

function renderDownloadCard(module, title, desc, manualPath, state, detected = null) {
  const job = state?.job || null;
  const pct = job && Number.isFinite(job.percent) ? Math.max(0, Math.min(100, Math.round(job.percent))) : 0;
  const detectedReady = detected && detected.status === 'ready';
  const detectedPartial = detected && detected.status === 'partial';
  const statusText = detectedReady
    ? '已就绪'
    : detectedPartial
      ? '部分就绪'
      : job ? `${job.status}${job.message ? ` / ${job.message}` : ''}` : '未开始';
  const err = job?.error ? `<div class="field-hint-error">${escapeHtml(job.error)}</div>` : '';
  const detectedPaths = detected?.paths && typeof detected.paths === 'object'
    ? `<div class="event-detail">已识别文件:<br>${Object.entries(detected.paths).map(([key, value]) => `${escapeHtml(key)} = <code>${escapeHtml(value)}</code>`).join('<br>')}</div>`
    : '';
  return `
    <div class="download-card">
      <div class="download-head">
        <strong>${escapeHtml(title)}</strong>
        <span>${escapeHtml(statusText)}</span>
      </div>
      <div class="event-detail">${escapeHtml(desc)}</div>
      <div class="event-detail">手动放置目录: <code>${escapeHtml(manualPath)}</code></div>
      ${detectedPaths}
      <div class="progress-wrap"><div class="progress-bar" style="width:${pct}%;"></div></div>
      <div class="event-detail">进度: ${pct}%</div>
      <div class="provider-actions">
        <button data-download-module="${escapeHtml(module)}" data-download-action="start" type="button" ${job && job.status === 'running' ? 'disabled' : ''}>开始下载</button>
        <button data-download-module="${escapeHtml(module)}" data-download-action="refresh" type="button">刷新进度</button>
        <button data-download-module="${escapeHtml(module)}" data-download-action="open-dir" type="button">打开目录</button>
      </div>
      ${err}
    </div>
  `;
}

function renderEmbeddedDownloadCards() {
  return '';
}

function renderDetailPanel() {
  if (!detailTitleEl || !detailSummaryEl || !detailConfigEl || !detailStatusEl || !draftSettings) return;
  const schema = moduleSchemas[selectedModule] || moduleSchemas.input;
  detailTitleEl.textContent = schema.title;
  detailSummaryEl.textContent = schema.summary;

  const status = moduleStatus(selectedModule);
  detailStatusEl.className = statusClass(status);
  detailStatusEl.textContent = statusLabel(status);

  const hideDraftButtons = selectedModule === 'model';
  if (detailApplyEl) detailApplyEl.style.display = hideDraftButtons ? 'none' : '';
  if (detailRevertEl) detailRevertEl.style.display = hideDraftButtons ? 'none' : '';
  if (detailDirtyEl) detailDirtyEl.style.display = hideDraftButtons ? 'none' : '';

  const formHTML = schema.fields.map((field) => {
    const value = getValueByPath(draftSettings, field.path);
    if (field.type === 'boolean') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><label><input data-path="${escapeHtml(field.path)}" type="checkbox" ${value ? 'checked' : ''} /></label></div>`;
    }
    if (field.type === 'select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(field.options || [], value)}</select></div>`;
    }
    if (field.type === 'model_select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(modelCatalog, value)}</select></div>`;
    }
    if (field.type === 'motion_select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(modelMotionGroups, value)}</select></div>`;
    }
    if (field.type === 'mic_device_select') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><select data-path="${escapeHtml(field.path)}">${optionListHTML(micDeviceOptions, value || 'default')}</select></div>`;
    }
    if (field.type === 'shared_endpoint') {
      const endpoint = String(value || '');
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><input data-path="${escapeHtml(field.path)}" type="text" value="${escapeHtml(endpoint)}" /></div>`;
    }
    if (field.type === 'textarea') {
      return `<div class="stat-line stat-line-textarea"><span>${escapeHtml(field.label)}</span><textarea data-path="${escapeHtml(field.path)}" rows="${Number(field.rows || 6)}">${escapeHtml(value ?? '')}</textarea></div>`;
    }
    if (field.type === 'password') {
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><input data-path="${escapeHtml(field.path)}" type="password" value="${escapeHtml(value ?? '')}" autocomplete="off" /></div>`;
    }
    if (field.type === 'model_name_select') {
      const current = String(value || '');
      const endpoint = endpointForField(field.path);
      const endpointState = endpointModelCache.get(endpoint) || { options: [], error: endpoint ? '模型列表未加载' : '模型接口地址为空' };
      let options = endpointState.options.length
        ? [...endpointState.options]
        : [];
      if (!current) {
        options = [{ value: '', label: '未选择模型' }, ...options];
      }
      if (current && !options.some((item) => String(item.value) === current)) {
        options = [{ value: current, label: `${current} (当前)` }, ...options];
      }
      const hint = endpointState.error ? `<div class="field-hint-error">${escapeHtml(endpointState.error)}</div>` : '';
      return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><div><div class="inline-actions"><select data-path="${escapeHtml(field.path)}">${optionListHTML(options, current)}</select><button data-model-action="refresh-models" data-model-path="${escapeHtml(field.path)}" type="button">刷新模型</button></div>${hint}</div></div>`;
    }
    const inputType = field.type === 'number' ? 'number' : 'text';
    return `<div class="stat-line"><span>${escapeHtml(field.label)}</span><input data-path="${escapeHtml(field.path)}" type="${inputType}" value="${escapeHtml(value ?? '')}" /></div>`;
  }).join('');

  const selectedMotion = String(getValueByPath(draftSettings, 'debug.motion_group') || modelMotionGroups[0] || 'Idle');
  const motionButtons = selectedModule === 'model'
    ? `<div class="model-action-grid">${modelMotionGroups.map((group) => `<button data-motion-group="${escapeHtml(group)}" type="button" class="${selectedMotion === String(group) ? 'primary' : ''}">${escapeHtml(group)}</button>`).join('')}</div>`
    : '';

  const modelActions = selectedModule === 'model'
    ? `<div class="model-action-grid"><button data-model-action="reload" type="button">重新加载</button></div>${motionButtons}`
    : '';

  detailConfigEl.innerHTML = `${formHTML}${modelActions}`;
  if (detailLLMTestEl) {
    detailLLMTestEl.style.display = selectedModule === 'llm' ? '' : 'none';
  }
}

function setDirty(nextDirty) {
  detailDirty = !!nextDirty;
  console.log('[debug-console] setDirty', { detailDirty, saving, selectedModule });
  if (detailDirtyEl) detailDirtyEl.textContent = detailDirty ? '有未保存改动' : '未修改';
  if (detailApplyEl) detailApplyEl.disabled = !detailDirty || saving;
  if (detailRevertEl) detailRevertEl.disabled = !detailDirty || saving;
}

function sanitizeSettingsForSave(cfg) {
  const clean = deepClone(cfg);
  if (clean.debug) delete clean.debug;
  const base = sanitizeSettingsForSave.base || {};
  const payload = {
    App: {
      Name: clean.app?.name,
      ModelName: clean.app?.model_name,
      Env: clean.app?.env,
      LogLevel: clean.app?.log_level,
      LogDir: clean.app?.log_dir
    },
    Database: {
      Path: clean.database?.path
    },
    Audio: {
      EnableWakeWord: clean.audio?.enable_wake_word,
      MicDeviceID: clean.audio?.mic_device_id,
      MicSampleRate: clean.audio?.mic_sample_rate,
      MicChannelCount: clean.audio?.mic_channel_count,
      WakeMode: clean.audio?.wake_mode,
      WakeProvider: clean.audio?.wake_provider,
      WakeBinaryPath: clean.audio?.wake_binary_path,
      WakeModelPath: clean.audio?.wake_model_path,
      WakeKeywordsFilePath: clean.audio?.wake_keywords_file_path,
      WakeKeyword: clean.audio?.wake_keyword,
      WakeWindowMs: clean.audio?.wake_window_ms,
      WakeDetectIntervalMs: clean.audio?.wake_detect_interval_ms,
      EndpointSilenceFrames: clean.audio?.endpoint_silence_frames,
      InterruptDuringSpeak: clean.audio?.interrupt_during_speaking,
      AECEnabled: clean.audio?.aec_enabled,
      NSEnabled: clean.audio?.ns_enabled,
      Scene: clean.audio?.scene
    },
    ASR: {
      Provider: clean.asr?.provider,
      Endpoint: clean.asr?.endpoint,
      StreamEndpoint: clean.asr?.stream_endpoint,
      RequestTimeoutMs: clean.asr?.request_timeout_ms,
      BinaryPath: clean.asr?.binary_path,
      ModelPath: clean.asr?.model_path,
      Language: clean.asr?.language,
      Threads: clean.asr?.threads
    },
    LLM: {
      Provider: clean.llm?.provider,
      Model: clean.llm?.model,
      Endpoint: clean.llm?.endpoint,
      RequestTimeoutMs: clean.llm?.request_timeout_ms,
      APIKey: clean.llm?.api_key,
      Persona: clean.llm?.persona
    },
    TTS: {
      Provider: clean.tts?.provider,
      Voice: clean.tts?.voice,
      Speed: clean.tts?.speed,
      Endpoint: clean.tts?.endpoint,
      RequestTimeoutMs: clean.tts?.request_timeout_ms,
      BinaryPath: clean.tts?.binary_path,
      ModelPath: clean.tts?.model_path,
      SentenceFirstEnabled: clean.tts?.sentence_first_enabled
    },
    RAG: {
      TopK: clean.rag?.top_k,
      VectorIndexPath: clean.rag?.vector_index_path,
      EmbedProvider: clean.rag?.embed_provider,
      EmbedEndpoint: clean.rag?.embed_endpoint,
      EmbedModel: clean.rag?.embed_model
    },
    Memory: {
      STMTokenBudget: clean.memory?.stm_token_budget,
      STMReserveImportant: clean.memory?.stm_reserve_important,
      SummaryAutoWrite: clean.memory?.summary_auto_write,
      GovernanceCompactHours: clean.memory?.governance_compact_hours
    },
    Perf: {},
    Security: {
      AllowRemoteProviders: clean.security?.allow_remote_providers,
      AllowedRemoteHosts: clean.security?.allowed_remote_hosts,
      ToolWhitelist: clean.security?.tool_whitelist,
      HighRiskConfirm: clean.security?.high_risk_confirm,
      PurgeProofDir: clean.security?.purge_proof_dir
    },
    Providers: {
      Root: clean.providers?.root
    }
  };
  return mergeWithFallback(base, payload);
}

async function saveSettings(nextDraft, successMessage) {
  if (saving) return;
  sanitizeSettingsForSave.base = sanitizeSettingsForSave.base || sanitizeSettingsForSave(draftSettings);
  const payload = sanitizeSettingsForSave(nextDraft);
  console.log('[debug-console] saveSettings:start', { selectedModule, detailDirty, rag: payload.RAG, llm: payload.LLM });
  log('保存配置开始', { module: selectedModule });
  saving = true;
  if (detailApplyEl) detailApplyEl.disabled = true;
  if (detailRevertEl) detailRevertEl.disabled = true;
  try {
    const resp = await fetch(`${api}/settings/runtime`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok || data?.ok !== true) {
      throw new Error(data?.error?.message || `保存配置失败: ${resp.status}`);
    }
    currentSettings = deepClone(nextDraft);
    draftSettings = deepClone(currentSettings);
    setDirty(false);
    log(successMessage);
    await loadAll(false);
  } catch (error) {
    logError('保存配置失败', error);
  } finally {
    saving = false;
    console.log('[debug-console] saveSettings:end', { selectedModule, detailDirty, saving });
    log('保存配置结束', { module: selectedModule });
    setDirty(detailDirty);
  }
}

function selectModule(moduleKey) {
  selectedModule = moduleKey;
  document.querySelectorAll('.console-row[data-module]').forEach((row) => {
    row.classList.toggle('active', row.dataset.module === selectedModule);
  });
  renderDetailPanel();
}

function bindSidebarModuleNav() {
  document.querySelectorAll('.console-row[data-module]').forEach((row) => {
    row.addEventListener('click', (event) => {
      if (event.target.closest('button,input,select')) return;
      const moduleKey = row.dataset.module;
      if (moduleKey) selectModule(moduleKey);
    });
  });
}

async function publishRuntimeEvent(name, payload) {
  const resp = await fetch(`${api}/runtime/publish`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, session_id: 'debug-console', trace_id: 'debug-console', payload })
  });
  if (!resp.ok) {
    const body = await resp.text().catch(() => '');
    throw new Error(`发送事件失败: ${name}${body ? ` -> ${body}` : ''}`);
  }
}

async function runModelAction(kind) {
  const modelURL = String(getValueByPath(draftSettings, 'debug.model_url') || (modelCatalog[0]?.url || '')).trim();
  const motion = String(getValueByPath(draftSettings, 'debug.motion_group') || 'Idle').trim();
  const motionIndex = Number(getValueByPath(draftSettings, 'debug.motion_index') || 0);

  if (kind === 'reload') {
    if (!modelURL) throw new Error('当前没有可用模型文件，请先在“模型文件”里选择有效项');
    localStorage.setItem(modelStorageKey, modelURL);
    await publishRuntimeEvent('live2d.reload', { source: 'debug-console', model_url: modelURL });
    log(`已触发模型重载: ${modelURL}`);
    return;
  }
  if (kind === 'motion') {
    await publishRuntimeEvent('live2d.motion', { group: motion, index: Number.isFinite(motionIndex) ? motionIndex : 0 });
    log(`已发送动作: ${motion}`);
    return;
  }
  if (kind === 'pose' || kind === 'mouth') return;
}


async function bindDetailFieldChange(event) {
    if (!draftSettings) return;
    const target = event.target;
    if (!target || !target.dataset?.path) return;
    const path = target.dataset.path;
    const schema = moduleSchemas[selectedModule];
    const field = schema?.fields?.find((item) => item.path === path);
    if (!field) return;

    let nextValue;
    if (field.type === 'boolean') {
      nextValue = !!target.checked;
    } else if (field.type === 'number') {
      const num = Number(target.value);
      if (!Number.isFinite(num)) return;
      nextValue = num;
    } else if (field.type === 'textarea') {
      nextValue = String(target.value || '');
    } else {
      nextValue = String(target.value || '').trim();
    }

    setValueByPath(draftSettings, path, nextValue);
    if (path === 'llm.endpoint') {
      const currentRAGEndpoint = String(getValueByPath(draftSettings, 'rag.embed_endpoint') || '').trim();
      const currentLLMEndpoint = String(getValueByPath(currentSettings, 'llm.endpoint') || '').trim();
      if (!currentRAGEndpoint || currentRAGEndpoint === currentLLMEndpoint) {
        setValueByPath(draftSettings, 'rag.embed_endpoint', String(nextValue || '').trim());
      }
      await loadEndpointModels(String(nextValue || '').trim());
    }
    if (path === 'rag.embed_endpoint') {
      await loadEndpointModels(String(nextValue || '').trim());
    }
    if (path === 'debug.model_url') {
      localStorage.setItem(modelStorageKey, nextValue);
      await loadMotionGroups(nextValue);
      const selected = modelCatalog.find((item) => String(item.url) === String(nextValue));
      const modelName = selected?.name || inferModelNameFromURL(nextValue);
      if (modelName) {
        if (!draftSettings.app || typeof draftSettings.app !== 'object') {
          draftSettings.app = {};
        }
        draftSettings.app.model_name = modelName;
        currentSettings = deepClone(draftSettings);
      await saveSettings(deepClone(draftSettings), `已切换模型目录：${modelName}`);
    } else {
      log('未能从模型路径推导 model_name，重启后可能不会保持该模型', { path: nextValue });
    }
    }
    if (!path.startsWith('debug.')) setDirty(true);
    renderSidebarText(currentSettings);
    renderSidebarIndicators();
    renderDetailPanel();
}

function bindDetailEditor() {
  detailConfigEl?.addEventListener('change', bindDetailFieldChange);
  detailConfigEl?.addEventListener('input', bindDetailFieldChange);

  detailConfigEl?.addEventListener('click', (event) => {
    const target = event.target;
    if (target?.dataset?.motionGroup) {
      setValueByPath(draftSettings, 'debug.motion_group', String(target.dataset.motionGroup));
      renderDetailPanel();
      runModelAction('motion').catch((error) => logError('触发模型动作失败', error));
      return;
    }
    if (target?.dataset?.modelAction === 'refresh-models') {
      const endpoint = endpointForField(String(target.dataset.modelPath || 'llm.model'));
      endpointModelCache.delete(endpoint);
      loadEndpointModels(endpoint)
        .then(() => {
          renderDetailPanel();
          log(`已刷新模型列表: ${endpoint || '-'}`);
        })
        .catch((error) => logError('刷新模型列表失败', error));
      return;
    }
    if (target?.dataset?.modelAction) {
      runModelAction(target.dataset.modelAction).catch((error) => logError('触发模型操作失败', error));
      return;
    }
    if (target?.dataset?.providerAction) {
      return;
    }
    if (target?.dataset?.downloadAction) {
      return;
    }
  });

  detailApplyEl?.addEventListener('click', async () => {
    console.log('[debug-console] detailApply:click', { selectedModule, detailDirty, hasDraft: !!draftSettings });
    if (!detailDirty || !draftSettings) return;
    await saveSettings(deepClone(draftSettings), `已应用 ${moduleSchemas[selectedModule]?.title || '模块'} 配置改动`);
  });

  detailRevertEl?.addEventListener('click', () => {
    if (!currentSettings) return;
    draftSettings = deepClone(currentSettings);
    ensureDebugDraft();
    setDirty(false);
    renderDetailPanel();
  });

  detailLLMTestEl?.addEventListener('click', () => {
    runLLMTestAction().catch((error) => logError('LLM \u6d4b\u8bd5\u5931\u8d25', error));
  });
}

async function loadDiscoverOnly() {
  discoverItems = [];
}

async function loadModelCatalog() {
  try {
    const resp = await fetch(`${api}/runtime/model/list`, { cache: 'no-store' });
    if (!resp.ok) throw new Error(`model list failed: ${resp.status}`);
    const data = await resp.json();
    const items = Array.isArray(data?.items) ? data.items : [];
    modelCatalog = items
      .filter((item) => item && typeof item.model_url === 'string' && item.model_url.trim() !== '')
      .map((item) => ({
        name: String(item.name || ''),
        label: String(item.label || item.name || '未命名模型'),
        url: `${api}${String(item.model_url).trim()}`
      }));
  } catch (_) {
    logError('读取模型列表失败', _);
    modelCatalog = [];
  }
}

async function resolveRuntimeModelURL() {
  try {
    const resp = await fetch(`${api}/runtime/model/resolve`, { cache: 'no-store' });
    if (!resp.ok) return '';
    const data = await resp.json();
    if (!data?.ok || typeof data.model_url !== 'string' || !data.model_url.trim()) return '';
    return `${api}${String(data.model_url).trim()}`;
  } catch (_) {
    logError('解析运行时模型路径失败', _);
    return '';
  }
}

async function loadAll(appendLog = true) {
  let settingsResp;
  try {
    settingsResp = await fetch(`${api}/settings/runtime`);
  } catch (error) {
    logError('拉取运行时配置失败', error);
    throw error;
  }
  if (!settingsResp.ok) throw new Error(`读取配置失败: ${settingsResp.status}`);

  const settings = normalizeRuntimeSettings(await settingsResp.json());
  sanitizeSettingsForSave.base = sanitizeSettingsForSave(settings);
  console.log('[debug-console] loadAll:settings', { rag: settings?.rag, llm: settings?.llm });
  await loadModelCatalog();
  await loadDiscoverOnly();

  if (!detailDirty || !currentSettings) {
    currentSettings = settings;
    draftSettings = deepClone(settings);
    ensureDebugDraft();
    setDirty(false);
  }
  sanitizeSettingsForSave.base = sanitizeSettingsForSave(settings);

  await loadMicDeviceOptions();
  endpointModelCache = new Map();
  await Promise.all([
    loadEndpointModels(getValueByPath(draftSettings, 'llm.endpoint')),
    loadEndpointModels(getValueByPath(draftSettings, 'rag.embed_endpoint'))
  ]);

  const resolvedModelURL = await resolveRuntimeModelURL();
  if (resolvedModelURL) {
    const current = String(getValueByPath(draftSettings, 'debug.model_url') || '').trim();
    if (!current || !modelCatalog.some((item) => String(item.url) === current)) {
      setValueByPath(draftSettings, 'debug.model_url', resolvedModelURL);
    }
  }
  await loadMotionGroups(getValueByPath(draftSettings, 'debug.model_url'));

  renderSidebarText(currentSettings);
  renderSidebarIndicators();
  renderDetailPanel();

  if (appendLog) log('控制台状态已刷新', { api });
}

document.getElementById('refresh')?.addEventListener('click', () => {
  loadAll().catch((error) => logError('刷新控制台失败', error));
});

bindSidebarModuleNav();
bindDetailEditor();
selectModule(selectedModule);
detectAPIBase()
  .then((base) => {
    api = base;
    return loadAll();
  })
  .catch((error) => logError('控制台初始化失败', error));
