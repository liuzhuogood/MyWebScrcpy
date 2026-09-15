(function () {
  'use strict';

  // ===== 依赖检查 =====
  // xterm.js 和 addon-fit 通过 CDN <script> 在 player.html 中加载。
  // 延迟初始化确保 DOM 和 xterm.js 均就绪。

  const serial = new URLSearchParams(location.search).get('serial');
  const playerMain = document.querySelector('.player-main');
  const btn = document.getElementById('btn-adb-shell');
  if (!btn || !playerMain || !serial) return;

  // ===== 状态 =====
  let panel = null;
  let resizer = null;
  let xtermWrap = null;
  let overlay = null;
  let overlayMsg = null;
  let reconnectBtn = null;
  let statusDot = null;

  let term = null;
  let fitAddon = null;
  let ws = null;
  let resizeObserver = null;
  let reconnectTimer = null;
  let reconnectAttempts = 0;
  const MAX_RECONNECT = 3;
  const RECONNECT_BASE_MS = 1500;

  let panelOpen = false;

  // ===== 构建面板 DOM =====
  function buildPanel() {
    // 头部
    const head = document.createElement('div');
    head.className = 'adb-shell-head';

    statusDot = document.createElement('span');
    statusDot.className = 'adb-shell-status-dot';

    const title = document.createElement('strong');
    title.append(statusDot, document.createTextNode(' ADB Shell'));

    const actions = document.createElement('div');
    actions.className = 'adb-shell-head-actions';

    const clearBtn = document.createElement('button');
    clearBtn.type = 'button';
    clearBtn.textContent = '清屏';
    clearBtn.title = '清屏';
    clearBtn.addEventListener('click', () => { if (term) term.clear(); });

    const closeBtn = document.createElement('button');
    closeBtn.type = 'button';
    closeBtn.textContent = '关闭';
    closeBtn.title = '关闭终端面板';
    closeBtn.addEventListener('click', closePanel);

    actions.append(clearBtn, closeBtn);
    head.append(title, actions);

    // xterm 容器
    xtermWrap = document.createElement('div');
    xtermWrap.className = 'adb-shell-xterm-wrap';

    // 断连覆盖层
    overlay = document.createElement('div');
    overlay.className = 'adb-shell-overlay';

    overlayMsg = document.createElement('div');
    overlayMsg.className = 'adb-shell-overlay-msg';

    reconnectBtn = document.createElement('button');
    reconnectBtn.type = 'button';
    reconnectBtn.textContent = '重新连接';
    reconnectBtn.addEventListener('click', () => {
      reconnectAttempts = 0;
      hideOverlay();
      connect();
    });

    overlay.append(overlayMsg, reconnectBtn);

    // 面板
    panel = document.createElement('section');
    panel.className = 'adb-shell-panel';
    panel.hidden = true;
    panel.setAttribute('role', 'region');
    panel.setAttribute('aria-label', 'ADB Shell 终端');
    panel.append(head, xtermWrap, overlay);

    // Resizer
    resizer = document.createElement('div');
    resizer.className = 'adb-shell-resizer';
    resizer.setAttribute('aria-hidden', 'true');

    playerMain.append(resizer, panel);
    setupResizer();
  }

  // ===== Resizer 拖拽 =====
  function setupResizer() {
    resizer.addEventListener('pointerdown', (e) => {
      if (innerWidth <= 820) return;
      e.preventDefault();
      const startX = e.clientX;
      const startWidth = panel.getBoundingClientRect().width;
      playerMain.classList.add('shell-resized');
      playerMain.style.setProperty('--shell-width', `${startWidth}px`);
      resizer.classList.add('dragging');
      resizer.setPointerCapture(e.pointerId);

      const move = (me) => {
        me.preventDefault();
        const maxWidth = Math.max(320, playerMain.clientWidth - 300);
        const width = Math.max(320, Math.min(maxWidth, startWidth + startX - me.clientX));
        playerMain.style.setProperty('--shell-width', `${width}px`);
        fitIfReady();
        window.dispatchEvent(new Event('resize'));
      };

      const stop = (se) => {
        resizer.classList.remove('dragging');
        resizer.removeEventListener('pointermove', move);
        resizer.removeEventListener('pointerup', stop);
        resizer.removeEventListener('pointercancel', stop);
        try { resizer.releasePointerCapture(e.pointerId); } catch (_) {}
        fitIfReady();
        window.dispatchEvent(new Event('resize'));
      };

      resizer.addEventListener('pointermove', move);
      resizer.addEventListener('pointerup', stop);
      resizer.addEventListener('pointercancel', stop);
    });
  }

  // ===== xterm.js 初始化 =====
  function initTerm() {
    if (term) { term.dispose(); term = null; }

    term = new Terminal({
      fontFamily: 'ui-monospace, "Cascadia Code", "Fira Code", monospace',
      fontSize: 13,
      lineHeight: 1.3,
      theme: {
        background:   '#18202d',
        foreground:   '#e8edf7',
        cursor:       '#e8edf7',
        black:        '#1c2433',
        red:          '#f53f3f',
        green:        '#00b42a',
        yellow:       '#ff7d00',
        blue:         '#165dff',
        magenta:      '#c07dff',
        cyan:         '#1cbfbf',
        white:        '#e8edf7',
        brightBlack:  '#465873',
        brightRed:    '#f96e6e',
        brightGreen:  '#4cd263',
        brightYellow: '#ffb239',
        brightBlue:   '#4e83fd',
        brightMagenta:'#d49dff',
        brightCyan:   '#5edede',
        brightWhite:  '#ffffff',
      },
      cursorBlink: true,
      scrollback: 5000,
      convertEol: false,
    });

    fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    term.open(xtermWrap);

    // 键盘输入 → WS
    term.onData((data) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });

    // 容器 resize → fit → 通知后端
    resizeObserver = new ResizeObserver(() => fitIfReady());
    resizeObserver.observe(xtermWrap);

    fitIfReady();
  }

  function fitIfReady() {
    if (!fitAddon || !term) return;
    try {
      fitAddon.fit();
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      }
    } catch (_) { /* ignore during teardown */ }
  }

  // ===== WebSocket 连接 =====
  function connect() {
    if (ws) {
      ws.onclose = null;
      ws.onerror = null;
      ws.close();
      ws = null;
    }

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${location.host}/ws/adb-shell?serial=${encodeURIComponent(serial)}`;

    setStatus('connecting');
    ws = new WebSocket(url);
    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
      reconnectAttempts = 0;
      setStatus('connected');
      hideOverlay();
      fitIfReady();
    };

    ws.onmessage = (event) => {
      if (!term) return;
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data));
      } else {
        // 文本帧（如错误消息）
        term.write(event.data);
      }
    };

    ws.onclose = () => {
      setStatus('error');
      scheduleReconnect();
    };

    ws.onerror = () => {
      // onclose 会紧随其后，不在此处重复处理
    };
  }

  function scheduleReconnect() {
    if (!panelOpen) return;
    if (reconnectAttempts >= MAX_RECONNECT) {
      showOverlay(`连接已断开，已重试 ${MAX_RECONNECT} 次`);
      return;
    }
    const delay = RECONNECT_BASE_MS * Math.pow(2, reconnectAttempts);
    reconnectAttempts++;
    showOverlay(`连接断开，${Math.round(delay / 1000)} 秒后重试（${reconnectAttempts}/${MAX_RECONNECT}）`, false);
    reconnectTimer = setTimeout(() => {
      if (panelOpen) connect();
    }, delay);
  }

  function setStatus(state) {
    if (!statusDot) return;
    statusDot.className = 'adb-shell-status-dot';
    if (state === 'connected') statusDot.classList.add('connected');
    if (state === 'error') statusDot.classList.add('error');
  }

  function showOverlay(msg, showBtn = true) {
    overlayMsg.textContent = msg;
    reconnectBtn.style.display = showBtn ? '' : 'none';
    overlay.classList.add('visible');
  }

  function hideOverlay() {
    overlay.classList.remove('visible');
  }

  // ===== 开/关面板 =====
  function openPanel() {
    // 互斥：关闭 inspector 面板（如果已打开）
    const inspectorBtn = document.getElementById('btn-inspector');
    if (inspectorBtn && inspectorBtn.getAttribute('aria-expanded') === 'true') {
      inspectorBtn.click();
    }
    // 互斥：关闭 RightPanel（文件管理、模板匹配）
    if (window.RightPanel) {
      window.RightPanel.close();
    }

    panel.hidden = false;
    playerMain.classList.remove('inspector-open', 'inspector-resized', 'shell-resized');
    playerMain.style.removeProperty('--shell-width');
    playerMain.classList.add('shell-open');
    panelOpen = true;
    btn.setAttribute('aria-expanded', 'true');
    closeMoreMenu();

    initTerm();
    reconnectAttempts = 0;
    connect();

    window.dispatchEvent(new Event('resize'));
  }

  function closePanel() {
    panelOpen = false;
    panel.hidden = true;
    playerMain.classList.remove('shell-open', 'shell-resized');
    playerMain.style.removeProperty('--shell-width');
    btn.setAttribute('aria-expanded', 'false');

    clearTimeout(reconnectTimer);
    if (ws) {
      ws.onclose = null;
      ws.onerror = null;
      ws.close();
      ws = null;
    }
    if (resizeObserver) { resizeObserver.disconnect(); resizeObserver = null; }
    if (term) { term.dispose(); term = null; }
    fitAddon = null;
    setStatus('disconnected');

    window.dispatchEvent(new Event('resize'));
  }

  function closeMoreMenu() {
    const menu = document.getElementById('more-menu');
    if (menu) menu.hidden = true;
    const moreBtn = document.getElementById('btn-more');
    if (moreBtn) moreBtn.setAttribute('aria-expanded', 'false');
  }

  // ===== 按钮绑定 =====
  buildPanel();
  btn.setAttribute('aria-expanded', 'false');
  btn.addEventListener('click', () => {
    if (!panelOpen) {
      openPanel();
    } else {
      closePanel();
    }
  });

  // inspector 打开时自动关闭 shell（互斥）
  const inspectorBtn = document.getElementById('btn-inspector');
  if (inspectorBtn) {
    inspectorBtn.addEventListener('click', () => {
      if (panelOpen && inspectorBtn.getAttribute('aria-expanded') === 'true') {
        // inspector 刚切换为 open，关闭 shell
        closePanel();
      }
    }, true /* capture，先于 inspector 自身处理 */);
  }

})();
