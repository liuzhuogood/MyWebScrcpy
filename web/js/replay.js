(() => {
  const params = new URLSearchParams(location.search);
  const serial = params.get('serial');
  const toolbar = document.querySelector('.player-toolbar');
  const tools = document.querySelector('.player-toolbar .tools');
  const mainEl = document.querySelector('.player-main');
  const screenWrap = document.querySelector('.player-main .screen-wrap');
  const moreButton = document.getElementById('btn-more');
  if (!toolbar || !tools || !serial || !mainEl) return;

  let replaying = false;
  let pollTimer = null;
  let tickTimer = null;
  let currentId = null;
  let sessionSeq = 0; // 本轮会话序号：start 成功自增，poll/finish 只处理自己那一轮，防止旧轮覆盖新会话
  let currentEntry = null; // 选中的 recording entry（含 started_at/ended_at）
  let totalMs = null; // ended_at - started_at
  let baseStartMs = 0; // 本轮本地计时起点（Date.now()）
  let lastLoopCount = null; // 后端 loop_count 上次值
  let lastLoop = false; // 后端/本次的 loop 值
  let pendingLoop = false; // 工具栏入口打开面板前的循环选择（默认 false）
  let totalFixTried = false; // 总时长未知时只补查一次录制信息，避免每轮 poll 都请求

  // ===== 工具栏入口按钮（面板开关，不再放徽标/停止按钮）=====
  const entryBtn = document.createElement('button');
  entryBtn.id = 'btn-replay';
  entryBtn.type = 'button';
  entryBtn.className = 'replay-entry-btn';
  entryBtn.title = '重放';
  entryBtn.setAttribute('aria-label', '重放');
  entryBtn.setAttribute('aria-haspopup', 'dialog');
  entryBtn.setAttribute('aria-expanded', 'false');
  entryBtn.innerHTML = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polygon points="5 4 15 12 5 20 5 4" fill="currentColor" stroke="none"/><line x1="19" y1="5" x2="19" y2="19"/></svg><span class="replay-entry-label">重放</span>';
  tools.insertBefore(entryBtn, moreButton);

  // ===== 右侧面板：放在 .player-main 内 screen-wrap 右侧，不遮挡画布主体 =====
  const panel = document.createElement('aside');
  panel.id = 'replay-panel';
  panel.className = 'replay-panel';
  panel.hidden = true;
  panel.setAttribute('role', 'dialog');
  panel.setAttribute('aria-label', '重放控制面板');
  panel.innerHTML = `
    <div class="replay-head"><strong>重放控制</strong><button id="replay-close" type="button" title="关闭" aria-label="关闭">×</button></div>
    <div class="replay-status-row"><span class="replay-loop-tag" hidden>循环</span><button id="replay-stop" class="replay-stop-inline" type="button" disabled>停止重放</button></div>
    <div class="replay-name" title="">--</div>
    <div class="replay-progress" role="progressbar" aria-label="重放进度" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0"><i></i></div>
    <div class="replay-times">
      <span>已播 <b data-k="elapsed">--</b></span>
      <span>总时长 <b data-k="total">--</b></span>
      <span>剩余 <b data-k="remain">--</b></span>
    </div>
    <label class="replay-loop"><input id="replay-loop" type="checkbox"><span>循环播放</span></label>
    <p class="replay-loop-hint" aria-live="polite"></p>
    <p class="replay-msg" aria-live="polite"></p>
    <section class="replay-list-wrap" aria-live="polite"><h2>录制列表</h2><p class="replay-list-empty">暂无录制记录</p><ul class="replay-list"></ul><p class="replay-list-msg" aria-live="polite"></p></section>`;
  if (screenWrap && screenWrap.parentNode === mainEl) screenWrap.after(panel);
  else mainEl.append(panel);

  const loopTag = panel.querySelector('.replay-loop-tag');
  const nameEl = panel.querySelector('.replay-name');
  let bar = panel.querySelector('.replay-progress');
  let barFill = panel.querySelector('.replay-progress i');
  let elapsedEl = panel.querySelector('[data-k="elapsed"]');
  let totalEl = panel.querySelector('[data-k="total"]');
  let remainEl = panel.querySelector('[data-k="remain"]');
  // 时间/进度条节点只在面板 innerHTML 初始化时创建一次，正常不会失效；
  // 这里做防御性重查：若某次外部渲染导致节点脱离文档，重建引用，保证 tick 仍能画上去。
  const refreshTimeRefs = () => {
    if (elapsedEl && elapsedEl.isConnected && barFill && barFill.isConnected) return;
    bar = panel.querySelector('.replay-progress');
    barFill = panel.querySelector('.replay-progress i');
    elapsedEl = panel.querySelector('[data-k="elapsed"]');
    totalEl = panel.querySelector('[data-k="total"]');
    remainEl = panel.querySelector('[data-k="remain"]');
  };
  const loopBox = panel.querySelector('#replay-loop');
  const loopHint = panel.querySelector('.replay-loop-hint');
  const stopBtn = panel.querySelector('#replay-stop');
  const closeBtn = panel.querySelector('#replay-close');
  const msgEl = panel.querySelector('.replay-msg');
  const listWrap = panel.querySelector('.replay-list-wrap');
  const listEl = panel.querySelector('.replay-list');
  const listEmpty = panel.querySelector('.replay-list-empty');
  const listMsg = panel.querySelector('.replay-list-msg');

  const toast = (msg) => {
    // 防自递归：__appToast 已是本 toast 时直接走本地实现
    if (window.__appToast && window.__appToast !== toast) { window.__appToast(msg); return; }
    let el = document.querySelector('.app-toast');
    if (!el) {
      el = document.createElement('div');
      el.className = 'app-toast';
      el.setAttribute('role', 'status');
      document.body.append(el);
    }
    el.textContent = msg;
    el.classList.add('show');
    clearTimeout(el._t);
    el._t = setTimeout(() => el.classList.remove('show'), 2600);
  };
  window.__appToast = window.__appToast || toast;

  const fmt = (ms) => {
    if (ms == null || !Number.isFinite(ms) || ms < 0) return '--';
    const s = Math.floor(ms / 1000);
    return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`;
  };
  const totalFromEntry = (entry) => {
    try {
      if (!entry || !entry.started_at || !entry.ended_at) return null;
      const ms = new Date(entry.ended_at) - new Date(entry.started_at);
      return Number.isFinite(ms) && ms > 0 ? ms : null;
    } catch (_) { return null; }
  };
  const entryName = (entry) => {
    if (!entry) return '--';
    try {
      if (entry.started_at) {
        return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(entry.started_at));
      }
    } catch (_) {}
    return entry.recording_id || '--';
  };

  // 录制列表的时间/时长文案复用 recording.js（window.__recordingFmt），无则本地同逻辑兜底
  const recTime = (entry) => {
    try {
      if (window.__recordingFmt && window.__recordingFmt.time) return window.__recordingFmt.time(entry);
    } catch (_) {}
    return entryName(entry);
  };
  const recDuration = (entry) => {
    try {
      if (window.__recordingFmt && window.__recordingFmt.duration) return window.__recordingFmt.duration(entry);
    } catch (_) {}
    const end = entry.ended_at ? new Date(entry.ended_at) : new Date();
    const seconds = Math.max(0, Math.round((end - new Date(entry.started_at)) / 1000));
    const minutes = Math.floor(seconds / 60);
    return minutes ? `${minutes} 分${String(seconds % 60).padStart(2, '0')} 秒` : `${seconds} 秒`;
  };

  const paintPlaying = () => {
    if (!listEl) return;
    listEl.querySelectorAll('li').forEach((li) => {
      const isPlaying = replaying && li.dataset.id === currentId;
      li.classList.toggle('is-playing', !!isPlaying);
      const del = li.querySelector('.replay-del-btn');
      if (del) {
        del.disabled = !!isPlaying;
        del.title = '删除这条录制记录';
      }
    });
  };

  async function loadReplayList() {
    if (!listEl || !serial) return;
    listMsg.textContent = '';
    listEmpty.hidden = false;
    listEmpty.textContent = '加载中…';
    try {
      const res = await fetch(`/api/recordings?serial=${encodeURIComponent(serial)}`);
      if (!res.ok) throw Error();
      const { recordings = [] } = await res.json();
      listEl.replaceChildren();
      listEmpty.hidden = recordings.length > 0;
      if (!recordings.length) listEmpty.textContent = '暂无录制记录';
      recordings.forEach((entry) => {
        const li = document.createElement('li');
        li.dataset.id = entry.recording_id;
        const details = document.createElement('div');
        details.className = 'replay-list-details';
        const name = document.createElement('strong');
        name.textContent = recTime(entry);
        name.title = entry.recording_id || '';
        const meta = document.createElement('span');
        meta.textContent = `录制 ${recDuration(entry)}${entry.record_audio ? ' · 含声音' : ' · 无声音'}`;
        details.append(name, meta);
        const actions = document.createElement('div');
        actions.className = 'replay-list-actions';
        if (entry.status === 'completed') {
          const replayBtn = document.createElement('button');
          replayBtn.type = 'button';
          replayBtn.textContent = '重放';
          replayBtn.className = 'replay-play-btn';
          replayBtn.onclick = () => startReplay(entry.recording_id, entry);
          actions.append(replayBtn);
          const download = document.createElement('a');
          download.href = `/api/recordings/download?serial=${encodeURIComponent(serial)}&recording_id=${encodeURIComponent(entry.recording_id)}`;
          download.download = '';
          download.textContent = '下载';
          actions.append(download);
        }
        if (!['recording', 'stopping'].includes(entry.status)) {
          const del = document.createElement('button');
          del.type = 'button';
          del.textContent = '删除';
          del.className = 'replay-del-btn';
          del.title = '删除这条录制记录';
          del.onclick = async () => {
            if (replaying && entry.recording_id === currentId) { toast('请先停止后再删除'); return; }
            if (!confirm('确定删除这条录制记录吗？')) return;
            del.disabled = true;
            try {
              const resp = await fetch(`/api/recordings/${encodeURIComponent(entry.recording_id)}?serial=${encodeURIComponent(serial)}`, { method: 'DELETE' });
              if (!resp.ok) throw Error();
              await loadReplayList();
            } catch (_) { listMsg.textContent = '删除失败，请重试'; }
            finally { paintPlaying(); }
          };
          actions.append(del);
        }
        li.append(details, actions);
        listEl.append(li);
      });
      paintPlaying();
    } catch (_) {
      listEl.replaceChildren();
      listEmpty.hidden = false;
      listEmpty.textContent = '录制列表加载失败，请重试';
    }
  }

  const emit = () => {
    document.dispatchEvent(new CustomEvent('replay:change', {
      detail: { replaying, recording_id: currentId, entry: currentEntry, loop: replaying ? lastLoop : loopBox.checked },
    }));
  };
  const switchStream = (msg) => {
    window.dispatchEvent(new CustomEvent('player:switch-stream', { detail: msg }));
  };
  const fitCanvas = () => {
    // player.html 的 fitCanvas 会在 resize/全屏时重算；面板显隐也触发布局
    window.dispatchEvent(new Event('resize'));
  };

  const setPanelOpen = (open) => {
    panel.hidden = !open;
    mainEl.classList.toggle('replay-open', open);
    entryBtn.setAttribute('aria-expanded', String(open));
    entryBtn.classList.toggle('is-active', open || replaying);
    if (open) {
      render(); loadReplayList();
      // 面板关闭再打开不杀计时；若 tick/poll 意外丢失（如异常分支清掉），重放中则重建，保证时间继续走
      if (replaying) {
        if (!baseStartMs) baseStartMs = Date.now();
        if (!tickTimer) startTick();
        if (!pollTimer) poll();
      }
    }
    fitCanvas();
  };

  // 当前已播：Date.now() - 本轮起点；loop 且总时长已知时本地取模，
  // 绕回瞬间自然清零，不依赖 2s 轮询发现 loop_count 变化（短片一遍不到 2s 会漏检）。
  // loop_count 变化仍在 poll 里对齐 baseStartMs 防漂移，显示以这里取模为准。
  const currentElapsed = () => {
    if (!replaying || !baseStartMs) return null;
    let ms = Date.now() - baseStartMs;
    if (ms < 0) ms = 0;
    if (totalMs) {
      if (lastLoop) ms = ms % totalMs;
      else if (ms > totalMs) ms = totalMs;
    }
    return ms;
  };

  function render() {
    try { refreshTimeRefs(); } catch (_) {}
    // 无提前 return 门控：重放中即使 totalMs 未知也要画已播时间；非重放态统一画 -- 并归零进度条
    // 状态隐式表达：停止按钮可用 + 时间在走 = 有重放；停止按钮禁用 + 时间 -- = 无重放
    const showLoop = replaying ? lastLoop : loopBox.checked;
    loopTag.hidden = !showLoop;
    nameEl.textContent = currentEntry ? entryName(currentEntry) : '--';
    nameEl.title = currentEntry ? (currentEntry.recording_id || '') : '';

    const total = totalMs;
    // 非重放态：已播、剩余一律显示 --，进度条归零
    const elapsed = replaying ? currentElapsed() : null;
    const remain = elapsed != null && total != null ? Math.max(0, total - elapsed) : null;
    elapsedEl.textContent = elapsed != null ? fmt(elapsed) : '--';
    totalEl.textContent = total != null ? fmt(total) : '--';
    remainEl.textContent = remain != null ? fmt(remain) : '--';
    const pct = elapsed != null && total ? Math.min(100, Math.max(0, (elapsed / total) * 100)) : 0;
    barFill.style.width = `${pct}%`;
    bar.setAttribute('aria-valuenow', String(Math.round(pct)));

    stopBtn.disabled = !replaying;
    // 重放进行中：循环开关禁用，下次重放生效
    loopBox.disabled = replaying;
    loopHint.textContent = replaying ? '重放进行中，循环设置下次重放生效' : '';
  }

  const startTick = () => {
    stopTick();
    // 单次 render 异常不杀计时器：包 try/catch，保证 500ms tick 持续空转画时间
    tickTimer = setInterval(() => { try { render(); } catch (_) {} }, 500);
  };
  const stopTick = () => { clearInterval(tickTimer); tickTimer = null; };

  const stopPoll = () => { clearTimeout(pollTimer); pollTimer = null; };
  const poll = () => {
    stopPoll();
    if (!replaying) return;
    const mySeq = sessionSeq;
    pollTimer = setTimeout(async () => {
      // 定时器已开火但本轮已结束（stop/finish/start 新一轮）：直接丢弃，不再收尾
      if (mySeq !== sessionSeq || !replaying) return;
      try {
        const res = await fetch(`/api/replay?serial=${encodeURIComponent(serial)}`);
        if (!res.ok) throw Error();
        // 在途请求返回时若已有新一轮（start 成功自增），旧轮结果直接丢弃
        if (mySeq !== sessionSeq || !replaying) return;
        const { replay } = await res.json();
        if (!replay) {
          // 非循环自然播完（后端删会话）：本轮才收尾；loop 模式后端不删会话，不会进这里
          await finishReplay('重放已播完，已回到实时流', mySeq);
          return;
        }
        // 后端契约：replay 带 loop，可选 loop_count
        if (typeof replay.loop === 'boolean') lastLoop = replay.loop;
        const n = replay.loop_count;
        if (typeof n === 'number') {
          if (lastLoopCount == null) lastLoopCount = n;
          else if (n !== lastLoopCount) {
            // loop=true 时后端播完重头：loop_count 变化即清零重计
            lastLoopCount = n;
            baseStartMs = Date.now();
          }
        } else {
          lastLoopCount = null; // 无该字段：显示照常按 render 内取模绕回
        }
        // entry 缺失导致总时长未知时，补查一次录制信息，避免剩余恒 --、进度恒 0
        if (totalMs == null && currentId && !totalFixTried) {
          totalFixTried = true;
          lookupEntry(currentId).then((found) => {
            if (found && replaying && totalMs == null) {
              if (!currentEntry) currentEntry = found;
              totalMs = totalFromEntry(found);
              render();
            }
          }).catch(() => {});
        }
        // 后端 started_at 若晚于本地起点（时钟差/重建），以首次值为准，不频繁重置
        render();
      } catch (_) { /* 后端暂未就绪或网络抖动：保持重放态，继续轮询 */ }
      // 只续自己那一轮：旧轮在途返回后不碰新会话的计时器
      if (mySeq !== sessionSeq || !replaying) return;
      poll();
    }, 2000);
  };

  const finishReplay = async (msg, seq) => {
    // 序号守卫：播完瞬间用户已 start 新一轮，旧轮收尾直接丢弃，不覆盖新会话
    if (seq !== undefined && seq !== sessionSeq) return;
    stopPoll(); stopTick();
    replaying = false;
    sessionSeq++; // 使在途旧 poll 失效
    currentId = null;
    currentEntry = null;
    totalMs = null;
    baseStartMs = 0;
    lastLoopCount = null;
    lastLoop = false;
    render(); emit(); paintPlaying();
    entryBtn.classList.toggle('is-active', !panel.hidden);
    switchStream('正在回到实时流…');
    if (msg) toast(msg);
  };

  async function lookupEntry(recordingId) {
    try {
      const res = await fetch(`/api/recordings?serial=${encodeURIComponent(serial)}`);
      if (!res.ok) return null;
      const { recordings = [] } = await res.json();
      return recordings.find((e) => e.recording_id === recordingId) || null;
    } catch (_) { return null; }
  }

  async function startReplay(recordingId, entry) {
    if (replaying) { toast('已在重放中，请先停止当前重放'); return; }
    if (!recordingId) { toast('缺少录像 ID，无法重放'); return; }
    const loop = !!loopBox.checked;
    try {
      // 后端新契约：POST /api/replay/start Body {"recording_id","loop":bool}
      const res = await fetch(`/api/replay/start?serial=${encodeURIComponent(serial)}`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ recording_id: recordingId, loop }),
      });
      // 失败分支不启动计时，且清掉可能残留的旧计时，避免“计时停摆后重进也不走”
      if (res.status === 404 || res.status === 501) { stopTick(); stopPoll(); toast('重放接口暂未就绪，请稍后重试'); return; }
      if (res.status === 409) { stopTick(); stopPoll(); toast('该设备正在录制或重放中，请稍后重试'); return; }
      if (!res.ok) throw Error();
      // 成功体为 replayView（含 started_at/loop/loop_count，可选）
      let view = null;
      try { view = await res.json(); } catch (_) { view = null; }
      replaying = true;
      sessionSeq++; // 新一轮：旧轮在途 poll/finish 全部失效
      currentId = recordingId;
      currentEntry = entry || await lookupEntry(recordingId);
      totalMs = totalFromEntry(currentEntry);
      totalFixTried = false;
      lastLoop = view && typeof view.loop === 'boolean' ? view.loop : loop;
      lastLoopCount = view && typeof view.loop_count === 'number' ? view.loop_count : null;
      // 已播计时起点：优先后端 started_at，否则本地 now
      let t = Date.parse(view && view.started_at);
      baseStartMs = Number.isFinite(t) ? t : Date.now();
      // 后端 started_at 是服务端时间：若晚于本地时钟（未来值）直接用本地 now，
      // 否则开头一段时间会被钳在 00:00 不动；与本地偏差超过 1 小时同样视为不可信
      if (baseStartMs > Date.now() || Math.abs(Date.now() - baseStartMs) > 3600 * 1000) baseStartMs = Date.now();
      setPanelOpen(true);
      render(); emit(); paintPlaying();
      switchStream('正在切换到重放…');
      toast(lastLoop ? '已开始重放（循环播放）' : '已开始重放');
      startTick(); poll();
    } catch (_) { stopTick(); stopPoll(); toast('开始重放失败，请重试'); }
  }

  async function stopReplay(manual = true) {
    if (!replaying && manual) return;
    const mySeq = sessionSeq;
    try {
      const res = await fetch(`/api/replay/stop?serial=${encodeURIComponent(serial)}`, { method: 'POST' });
      // 404 = 后端已无重放（如短片已播完自停）：本地照常收尾，避免 replaying 卡死导致下次无法重放
      if (res.status === 404) { await finishReplay(manual ? '重放已结束，回到实时流' : undefined, mySeq); return; }
      if (res.status === 501) { toast('重放接口暂未就绪，请稍后重试'); return; }
      if (!res.ok && res.status !== 409) throw Error();
    } catch (_) { toast('停止重放失败，请重试'); return; }
    await finishReplay(manual ? '已停止重放，回到实时流' : undefined, mySeq);
  }

  entryBtn.addEventListener('click', () => setPanelOpen(panel.hidden));
  closeBtn.addEventListener('click', () => setPanelOpen(false));
  stopBtn.addEventListener('click', () => stopReplay(true));
  // 页签切后台时浏览器会节流 setInterval，回来后按墙钟（Date.now 差值）立即追齐显示
  document.addEventListener('visibilitychange', () => { if (!document.hidden) render(); });
  window.addEventListener('pageshow', () => render());
  window.addEventListener('focus', () => render());
  loopBox.addEventListener('change', () => {
    pendingLoop = loopBox.checked;
    render(); emit();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !panel.hidden) setPanelOpen(false);
  });

  render();
  window.__replayStart = startReplay;
  window.__replayStop = () => stopReplay(true);
  window.__isReplaying = () => replaying;
})();
