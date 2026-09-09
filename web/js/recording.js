(() => {
  const serial = new URLSearchParams(location.search).get('serial');
  const tools = document.querySelector('.player-toolbar .tools');
  const more = document.getElementById('more-menu');
  const moreButton = document.getElementById('btn-more');
  const record = document.createElement('button');
  record.id = 'btn-record'; record.className = 'record-btn'; record.title = '录制屏幕';
  record.setAttribute('aria-haspopup', 'dialog');
  record.innerHTML = '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="12" cy="12" r="7"/></svg><span class="record-label">录制</span>';
  tools.insertBefore(record, moreButton);

  const panel = document.createElement('div');
  panel.id = 'recording-panel'; panel.className = 'recording-panel'; panel.hidden = true;
  panel.setAttribute('role', 'dialog'); panel.setAttribute('aria-label', '录制设置和记录');
  panel.innerHTML = `
    <section class="recording-new">
      <label>录制时长 <output>30 分钟</output></label>
      <input type="range" min="1" max="480" value="30" step="1">
      <div class="recording-panel-actions"><button id="recording-start">开始录制</button><button id="recording-cancel">取消</button></div>
      <p class="recording-state" aria-live="polite"></p>
    </section>
    <section class="recording-history" aria-live="polite"><h2>录制记录</h2><p class="recording-history-empty">暂无录制记录</p><ul></ul></section>`;
  document.querySelector('.player-toolbar').append(panel);

  const range = panel.querySelector('input');
  const output = panel.querySelector('output');
  const start = panel.querySelector('#recording-start');
  const cancel = panel.querySelector('#recording-cancel');
  const state = panel.querySelector('.recording-state');
  const history = panel.querySelector('.recording-history');
  const historyList = history.querySelector('ul');
  const historyEmpty = history.querySelector('.recording-history-empty');
  let active = null;
  let timer;

  const duration = minutes => minutes >= 60 ? `${Math.floor(minutes / 60)} 小时${minutes % 60 ? ` ${minutes % 60} 分钟` : ''}` : `${minutes} 分钟`;
  const recordingDuration = entry => {
    const end = entry.ended_at ? new Date(entry.ended_at) : new Date();
    const seconds = Math.max(0, Math.round((end - new Date(entry.started_at)) / 1000));
    const minutes = Math.floor(seconds / 60);
    return minutes ? `${minutes} 分${String(seconds % 60).padStart(2, '0')} 秒` : `${seconds} 秒`;
  };
  const recordingTime = entry => new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(entry.started_at));
  const close = () => { panel.hidden = true; record.setAttribute('aria-expanded', 'false'); };

  const render = () => {
    const running = active && ['recording', 'stopping'].includes(active.status);
    record.classList.toggle('is-recording', running);
    record.querySelector('span').textContent = active?.status === 'recording' ? '停止录制' : active?.status === 'stopping' ? '停止中' : '录制';
    if (active?.status === 'recording') state.textContent = `录制中，已录制 ${recordingDuration(active)}`;
    else if (active?.status === 'stopping') state.textContent = '正在完成录像…';
    else if (active?.status === 'failed') state.textContent = '录制未完成，请重新开始。';
    else if (active?.status === 'completed') state.textContent = '录制完成，已加入下方记录。';
  };

  const loadHistory = async () => {
    if (!serial) return;
    try {
      const response = await fetch(`/api/recordings?serial=${encodeURIComponent(serial)}`);
      if (!response.ok) throw Error();
      const { recordings = [] } = await response.json();
      historyList.replaceChildren();
      historyEmpty.hidden = recordings.length > 0;
      recordings.forEach(entry => {
        const item = document.createElement('li');
        const details = document.createElement('div');
        details.innerHTML = `<strong>${recordingTime(entry)}</strong><span>录制 ${recordingDuration(entry)}</span>`;
        const actions = document.createElement('div'); actions.className = 'recording-history-actions';
        if (entry.status === 'completed') {
          const download = document.createElement('a');
          download.href = `/api/recordings/download?serial=${encodeURIComponent(serial)}&recording_id=${encodeURIComponent(entry.recording_id)}`;
          download.download = ''; download.textContent = '下载'; actions.append(download);
        }
        if (!['recording', 'stopping'].includes(entry.status)) {
          const remove = document.createElement('button'); remove.type = 'button'; remove.textContent = '删除';
          remove.onclick = async () => {
            if (!confirm('确定删除这条录制记录吗？')) return;
            remove.disabled = true;
            try {
              const response = await fetch(`/api/recordings/${encodeURIComponent(entry.recording_id)}?serial=${encodeURIComponent(serial)}`, { method: 'DELETE' });
              if (!response.ok) throw Error();
              await loadHistory();
            } catch (_) { state.textContent = '暂时无法删除录制记录，请重试。'; }
            finally { remove.disabled = false; }
          };
          actions.append(remove);
        }
        item.append(details, actions); historyList.append(item);
      });
    } catch (_) { historyEmpty.hidden = false; historyEmpty.textContent = '暂时无法获取录制记录'; }
  };

  const poll = () => {
    clearTimeout(timer);
    if (!active || !['recording', 'stopping'].includes(active.status)) return;
    timer = setTimeout(async () => {
      try {
        const response = await fetch(`/api/recordings/${encodeURIComponent(active.recording_id)}?serial=${encodeURIComponent(serial)}`);
        if (response.ok) active = await response.json();
      } catch (_) {}
      render();
      if (!['recording', 'stopping'].includes(active?.status)) await loadHistory();
      poll();
    }, 1000);
  };

  range.oninput = () => { output.textContent = duration(+range.value); };
  range.addEventListener('keydown', event => event.stopPropagation());
  cancel.onclick = close;
  record.onclick = async () => {
    if (active?.status === 'recording') {
      record.disabled = true;
      try {
        const response = await fetch(`/api/recordings/${encodeURIComponent(active.recording_id)}/stop?serial=${encodeURIComponent(serial)}`, { method: 'POST' });
        if (!response.ok) throw Error();
        active = await response.json(); close();
      } catch (_) { state.textContent = '暂时无法停止录制，请重试。'; }
      finally { record.disabled = false; render(); poll(); }
      return;
    }
    panel.hidden = !panel.hidden;
    record.setAttribute('aria-expanded', String(!panel.hidden));
    if (!panel.hidden) loadHistory();
  };
  start.onclick = async () => {
    start.disabled = true; state.textContent = '正在开始录制…';
    try {
      const response = await fetch(`/api/recordings?serial=${encodeURIComponent(serial)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ max_duration_ms: +range.value * 60000 }) });
      if (!response.ok) throw Error();
      active = await response.json(); close(); await loadHistory();
    } catch (_) { state.textContent = '暂时无法开始录制，请稍后重试。'; }
    finally { start.disabled = false; render(); poll(); }
  };

  const all = ['btn-power', 'btn-recents', 'btn-rotate', 'btn-files', 'btn-fullscreen', 'btn-raw-size', 'btn-capture', 'btn-reboot'].map(id => document.getElementById(id)).filter(Boolean);
  const low = all.filter(button => !['btn-fullscreen', 'btn-capture'].includes(button.id));
  const layout = () => {
    more.hidden = true; all.forEach(button => tools.insertBefore(button, moreButton)); moreButton.hidden = true;
    for (const button of [...low].reverse()) {
      if (tools.scrollWidth <= tools.clientWidth) break;
      more.append(button); moreButton.hidden = false;
    }
  };
  new ResizeObserver(() => requestAnimationFrame(layout)).observe(tools);
  requestAnimationFrame(layout);
})();
