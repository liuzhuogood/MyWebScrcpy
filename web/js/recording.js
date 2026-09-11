(() => {
  const serial = new URLSearchParams(location.search).get('serial');
  const tools = document.querySelector('.player-toolbar .tools');
  const more = document.getElementById('more-menu');
  const moreButton = document.getElementById('btn-more');
  const record = document.createElement('button');
  record.id = 'btn-record'; record.className = 'record-btn'; record.title = '录制'; record.type = 'button';
  record.setAttribute('aria-label', '录制');
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
	  <label class="recording-audio-option"><span>同时录制设备声音</span><input id="recording-audio" type="checkbox"></label>
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

  const recordAudio = panel.querySelector('#recording-audio');
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
  window.__recordingFmt = Object.assign(window.__recordingFmt || {}, { time: recordingTime, duration: recordingDuration });
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
        details.innerHTML = `<strong>${recordingTime(entry)}</strong><span>录制 ${recordingDuration(entry)}${entry.record_audio ? ' · 含声音' : ' · 无声音'}</span>`;
        const actions = document.createElement('div'); actions.className = 'recording-history-actions';
        if (entry.status === 'completed') {
          const download = document.createElement('a');
          download.href = `/api/recordings/download?serial=${encodeURIComponent(serial)}&recording_id=${encodeURIComponent(entry.recording_id)}`;
          download.download = ''; download.textContent = '下载'; actions.append(download);
          const replay = document.createElement('button');
          replay.type = 'button'; replay.textContent = '重放'; replay.className = 'recording-replay-btn';
          replay.onclick = () => {
            if (window.__isReplaying && window.__isReplaying()) { state.textContent = '已在重放中，请先停止当前重放。'; return; }
            if (window.__replayStart) window.__replayStart(entry.recording_id, entry);
            else state.textContent = '重放功能暂未就绪，请稍后重试。';
          };
          actions.append(replay);
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
    if (!panel.hidden) {
      window.placeToolbarPopover(panel, record);
      loadHistory();
    }
  };
  start.onclick = async () => {
    if (document.documentElement.dataset.replaying === '1') { state.textContent = '重放期间无法开始录制，请先停止重放。'; return; }
    start.disabled = true; state.textContent = '正在开始录制…';
    try {
      const response = await fetch(`/api/recordings?serial=${encodeURIComponent(serial)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ max_duration_ms: +range.value * 60000, record_audio: recordAudio.checked }) });
      if (!response.ok) { const detail = await response.json().catch(() => ({})); throw Error(detail.message || '暂时无法开始录制，请稍后重试。'); }
      active = await response.json(); close(); await loadHistory();
    } catch (error) { state.textContent = error.message || '暂时无法开始录制，请稍后重试。'; }
    finally { start.disabled = false; render(); poll(); }
  };

  document.addEventListener('replay:change', (event) => {
    const replaying = !!event.detail?.replaying;
    document.documentElement.dataset.replaying = replaying ? '1' : '';
    start.disabled = replaying;
    if (replaying) state.textContent = '重放期间无法开始录制，请先停止重放。';
    else if (state.textContent === '重放期间无法开始录制，请先停止重放。') state.textContent = '';
  });

  // 工具栏最终顺序（静态 + 动态按钮统一在此排序）：
  // 返回 → 主页 → 任务切换 → 音量 → 锁屏 → 录制 → 重放 → 文件管理 →
  // 截图 → 区域截图 → 全屏 → 画中画 → 显示尺寸 → 横竖屏 → 检查 → 重启设备
  const ORDER = ['btn-back', 'btn-home', 'btn-recents', 'btn-audio', 'btn-power', 'btn-record', 'btn-replay', 'btn-files', 'btn-screenshot', 'btn-capture', 'btn-fullscreen', 'btn-picture-in-picture', 'btn-display-size', 'btn-rotate', 'btn-inspector', 'btn-reboot'];
  // 常驻工具栏、窄屏也不收进“更多”：返回/主页（导航）+ 全屏/区域截图（沿用原有行为）
  const PINNED = new Set(['btn-back', 'btn-home', 'btn-fullscreen', 'btn-capture']);
  const layout = () => {
    const buttons = ORDER.map(id => document.getElementById(id)).filter(Boolean);
    more.hidden = true; buttons.forEach(button => tools.insertBefore(button, moreButton)); moreButton.hidden = true;
    for (const button of [...buttons].reverse()) {
      if (PINNED.has(button.id)) continue;
      if (tools.scrollWidth <= tools.clientWidth) break;
      more.prepend(button); moreButton.hidden = false;
    }
  };
  new ResizeObserver(() => requestAnimationFrame(layout)).observe(tools);
  requestAnimationFrame(layout);
})();
