const api = 'http://127.0.0.1:8090';
const rowsEl = document.getElementById('rows');
const logEl = document.getElementById('log');
const byId = (id) => document.getElementById(id);
const log = (s) => { logEl.textContent = `[${new Date().toLocaleTimeString()}] ${s}\n` + logEl.textContent; };

async function list() {
  const resp = await fetch(`${api}/memory?include_disabled=1&limit=100`);
  if (!resp.ok) throw new Error(`list failed: ${resp.status}`);
  const data = await resp.json();
  rowsEl.innerHTML = data.map((item) => `
    <tr>
      <td>${item.id}</td>
      <td>${item.content || ''}</td>
      <td>${item.memory_type || item.kind || ''}</td>
      <td>${item.importance ?? 0}</td>
      <td>${item.disabled ? '已禁用' : '启用中'}</td>
      <td>
        <button data-id="${item.id}" data-action="toggle">${item.disabled ? '启用' : '禁用'}</button>
        <button data-id="${item.id}" data-action="delete">删除</button>
      </td>
    </tr>
  `).join('');
}

async function create() {
  const payload = {
    content: byId('content').value.trim(),
    kind: byId('kind').value.trim(),
    importance: Number(byId('importance').value || 5),
    memory_type: byId('memory_type').value.trim(),
    source: byId('source').value.trim(),
    evidence: byId('evidence').value.trim()
  };
  const resp = await fetch(`${api}/memory`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  if (!resp.ok) throw new Error(`create failed: ${resp.status}`);
  log('记忆已创建。');
  await list();
}

async function governance() {
  const resp = await fetch(`${api}/memory/governance/run`, { method: 'POST' });
  if (!resp.ok) throw new Error(`governance failed: ${resp.status}`);
  const data = await resp.json();
  log(`治理完成: ${JSON.stringify(data)}`);
  await list();
}

async function updateDisabled(id, disabled) {
  const resp = await fetch(`${api}/memory/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ disabled })
  });
  if (!resp.ok) throw new Error(`toggle failed: ${resp.status}`);
  await list();
}

async function remove(id) {
  const resp = await fetch(`${api}/memory/${id}`, { method: 'DELETE' });
  if (!resp.ok) throw new Error(`delete failed: ${resp.status}`);
  await list();
}

rowsEl.addEventListener('click', async (event) => {
  const button = event.target.closest('button');
  if (!button) return;
  const id = button.dataset.id;
  const action = button.dataset.action;
  try {
    if (action === 'toggle') {
      const disabled = button.textContent === '禁用';
      await updateDisabled(id, disabled);
      log(`记忆 ${id} 状态已切换。`);
      return;
    }
    if (action === 'delete') {
      await remove(id);
      log(`记忆 ${id} 已删除。`);
    }
  } catch (err) {
    log(String(err));
  }
});

byId('create').addEventListener('click', () => create().catch((e) => log(String(e))));
byId('governance').addEventListener('click', () => governance().catch((e) => log(String(e))));
byId('refresh').addEventListener('click', () => list().catch((e) => log(String(e))));

list().catch((e) => log(String(e)));