// op-log.js — 操作日志模块
// 提供 window.OperationLog 全局单例
// 由 ui-inspector.js 调用 init(container) 挂载 DOM
// 由 player.html 调用 record(type, data) 写入日志
(function () {
  'use strict';

  const MAX_ENTRIES = 500;
  const serial = new URLSearchParams(location.search).get('serial') || '';

  let entries = [];
  let mode = 'desc'; // 'desc' | 'adb'
  let listEl = null;
  let modeDescBtn = null;
  let modeAdbBtn = null;

  /* ---- SVG 图标 ---- */
  const SVG_COPY = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`;
  const SVG_CHECK = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`;
  const SVG_DEL = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>`;

  /* ---- ADB 前缀 ---- */
  function adbPrefix() {
    return serial ? `adb -s '${serial}' shell input` : 'adb shell input';
  }

  /* ---- 写入一条日志 ---- */
  function record(type, data) {
    const now = new Date();
    const time = now.toTimeString().slice(0, 8); // HH:MM:SS
    let desc = '';
    let adb = '';

    if (type === 'tap') {
      desc = `点击 (${data.x}, ${data.y})`;
      adb  = `${adbPrefix()} tap ${data.x} ${data.y}`;
    } else if (type === 'longpress') {
      desc = `长按 (${data.x}, ${data.y})  ${data.duration}ms`;
      adb  = `${adbPrefix()} swipe ${data.x} ${data.y} ${data.x} ${data.y} ${data.duration}`;
    } else if (type === 'swipe') {
      desc = `滑动 (${data.x1}, ${data.y1}) → (${data.x2}, ${data.y2})  ${data.duration}ms`;
      adb  = `${adbPrefix()} swipe ${data.x1} ${data.y1} ${data.x2} ${data.y2} ${data.duration}`;
    } else if (type === 'key') {
      desc = `按键 ${data.label}`;
      adb  = `${adbPrefix()} keyevent ${data.keycode}`;
    } else if (type === 'text') {
      const escaped = data.text.replace(/"/g, '\\"');
      desc = `输入 "${data.text}"`;
      adb  = `${adbPrefix()} text "${escaped}"`;
    } else {
      return;
    }

    const entry = { time, desc, adb };
    entries.push(entry);
    if (entries.length > MAX_ENTRIES) entries.shift();
    appendEntry(entry);
  }

  /* ---- 追加一行到 DOM ---- */
  function appendEntry(entry) {
    if (!listEl) return;

    // 移除占位提示
    const ph = listEl.querySelector('.ui-oplog-placeholder');
    if (ph) ph.remove();

    const row = document.createElement('div');
    row.className = 'ui-oplog-row';

    // 文本区（描述 / ADB 各一个 span，靠 CSS mode 类切换）
    const descSpan = document.createElement('span');
    descSpan.className = 'ui-oplog-text ui-oplog-desc';
    descSpan.textContent = `[${entry.time}] ${entry.desc}`;

    const adbSpan = document.createElement('span');
    adbSpan.className = 'ui-oplog-text ui-oplog-adb';
    adbSpan.textContent = entry.adb;

    // 操作按钮区
    const actions = document.createElement('span');
    actions.className = 'ui-oplog-actions';

    // 复制按钮
    const copyBtn = document.createElement('button');
    copyBtn.className = 'ui-oplog-btn';
    copyBtn.title = '复制';
    copyBtn.innerHTML = SVG_COPY;
    copyBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      const text = mode === 'adb' ? entry.adb : entry.desc;
      navigator.clipboard?.writeText(text).then(() => {
        copyBtn.innerHTML = SVG_CHECK;
        copyBtn.classList.add('ui-oplog-btn-ok');
        setTimeout(() => {
          copyBtn.innerHTML = SVG_COPY;
          copyBtn.classList.remove('ui-oplog-btn-ok');
        }, 900);
      }).catch(() => {});
    });

    // 删除按钮
    const delBtn = document.createElement('button');
    delBtn.className = 'ui-oplog-btn ui-oplog-btn-del';
    delBtn.title = '删除';
    delBtn.innerHTML = SVG_DEL;
    delBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      const idx = entries.indexOf(entry);
      if (idx !== -1) entries.splice(idx, 1);
      row.remove();
      if (entries.length === 0) renderPlaceholder();
    });

    actions.append(copyBtn, delBtn);
    row.append(descSpan, adbSpan, actions);
    listEl.append(row);
    listEl.scrollTop = listEl.scrollHeight;
  }

  /* ---- 显示占位提示 ---- */
  function renderPlaceholder() {
    if (!listEl) return;
    listEl.replaceChildren();
    const ph = document.createElement('div');
    ph.className = 'ui-oplog-placeholder ui-inspector-message';
    ph.textContent = '操作画面后将在此显示日志';
    listEl.append(ph);
  }

  /* ---- 切换描述 / ADB 模式 ---- */
  function setMode(newMode) {
    mode = newMode;
    if (listEl) listEl.classList.toggle('mode-adb', mode === 'adb');
    if (modeDescBtn) modeDescBtn.classList.toggle('active', mode === 'desc');
    if (modeAdbBtn) modeAdbBtn.classList.toggle('active', mode === 'adb');
  }

  /* ---- 清空日志 ---- */
  function clear() {
    entries = [];
    renderPlaceholder();
  }

  /* ---- 复制全部 ---- */
  function copyAll() {
    if (!entries.length) return;
    const lines = entries.map(e => mode === 'adb' ? e.adb : `[${e.time}] ${e.desc}`);
    navigator.clipboard?.writeText(lines.join('\n')).catch(() => {});
  }

  /* ---- 由 ui-inspector.js 调用，挂载到 tab 容器 ---- */
  function init(container) {
    const controls = document.createElement('div');
    controls.className = 'ui-inspector-controls';

    modeDescBtn = document.createElement('button');
    modeDescBtn.textContent = '描述';
    modeDescBtn.classList.add('active');
    modeDescBtn.onclick = () => setMode('desc');

    modeAdbBtn = document.createElement('button');
    modeAdbBtn.textContent = 'ADB';
    modeAdbBtn.onclick = () => setMode('adb');

    const copyAllBtn = document.createElement('button');
    copyAllBtn.textContent = '复制全部';
    copyAllBtn.onclick = copyAll;

    const clearBtn = document.createElement('button');
    clearBtn.textContent = '清空';
    clearBtn.onclick = clear;

    controls.append(modeDescBtn, modeAdbBtn, copyAllBtn, clearBtn);

    listEl = document.createElement('div');
    listEl.className = 'ui-oplog-list';

    renderPlaceholder();
    container.append(controls, listEl);
  }

  window.OperationLog = { record, init, clear };
}());
