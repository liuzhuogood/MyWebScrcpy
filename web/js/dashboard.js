// dashboard.js — 大屏监控页逻辑
//
// 同时管理多台设备的 WebSocket 连接，每台设备独立解码和渲染，
// 并支持基础触控操作（点击、滑动、滚轮）。
// 复用 ScrcpyDecoder 和 ControlPacker。

// ===== 设备别名 localStorage（与 index.html 保持一致）=====
function getDeviceAliases() {
  try {
    return JSON.parse(localStorage.getItem('device_aliases') || '{}');
  } catch {
    return {};
  }
}

function getDeviceAlias(serial) {
  return getDeviceAliases()[serial] || '';
}

// ===== 设备连接管理 =====
// 每台设备创建一个 DeviceClient，负责 WebSocket 连接、解码和触控。
class DeviceClient {
  constructor(serial, cardEl) {
    this.serial = serial;
    this.cardEl = cardEl;
    this.canvas = cardEl.querySelector('canvas');
    this.statusEl = cardEl.querySelector('.dash-status');
    this.overlay = cardEl.querySelector('.dash-overlay');
    this.wrap = cardEl.querySelector('.dash-screen-wrap');

    this.decoder = new ScrcpyDecoder(this.canvas);
    this.packer = new ControlPacker();
    this.ws = null;
    this.connected = false;
    this.mouseDown = false;
    this.reconnectAttempts = 0;
    this.reconnectTimer = null;
    this.destroyed = false;

    this.decoder.onFps = () => {}; // 大屏不显示 fps
    this.decoder.onDeviceResize = (w, h) => {
      this.packer.setDeviceSize(w, h);
      this.updateOrientation(w, h);
      this.fitCanvas();
    };

    this.bindTouchEvents();
    this.bindToolbar();
    this.observeResize();
    this.connect();
  }

  // 监听屏幕区域尺寸变化，自动重算 canvas
  // 覆盖切尺寸、窗口 resize、横竖屏切换等所有场景
  observeResize() {
    const ro = new ResizeObserver(() => this.fitCanvas());
    ro.observe(this.wrap);
    this._resizeObserver = ro;
  }

  setStatus(text, cls) {
    // 直播为正常状态，大屏上不再显示"直播中"徽章
    if (cls === 'live') {
      this.statusEl.style.display = 'none';
      return;
    }
    this.statusEl.style.display = '';
    this.statusEl.textContent = text;
    this.statusEl.className = 'dash-status' + (cls ? ' ' + cls : '');
  }

  showLoading(text = '连接中...') {
    this.overlay.className = 'dash-overlay loading';
    this.overlay.innerHTML = `<div class="spinner"></div><div class="overlay-text">${text}</div>`;
  }

  showError(msg) {
    this.overlay.className = 'dash-overlay error';
    this.overlay.innerHTML = `
      <div class="overlay-text">${msg}</div>
      <a class="open-link" href="/player.html?serial=${encodeURIComponent(this.serial)}">在播放器中打开 →</a>
    `;
  }

  hideOverlay() {
    this.overlay.style.display = 'none';
  }

  connect() {
    if (this.destroyed) return;
    const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${wsProto}//${location.host}/ws?serial=${encodeURIComponent(this.serial)}`;
    this.ws = new WebSocket(wsUrl);
    this.ws.binaryType = 'arraybuffer';
    this.showLoading();

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.setStatus('等待画面', '');
    };

    this.ws.onmessage = (event) => {
      this.handleMessage(event);
    };

    this.ws.onerror = () => {};

    this.ws.onclose = () => {
      this.connected = false;
      if (this.destroyed) return;
      if (this.reconnectAttempts < 10) {
        this.reconnect();
      } else {
        this.setStatus('连接失败', 'error');
        this.showError('连接失败');
      }
    };
  }

  handleMessage(event) {
    try {
      if (typeof event.data === 'string') {
        const msg = JSON.parse(event.data);
        if (msg.type === 'meta') {
          this.packer.setDeviceSize(msg.width, msg.height);
          this.decoder.configure(msg.codec, msg.width, msg.height);
          this.connected = true;
          this.setStatus('直播中', 'live');
          this.hideOverlay();
          this.updateOrientation(msg.width, msg.height);
          this.fitCanvas();
        } else if (msg.type === 'error') {
          this.setStatus('错误', 'error');
          this.showError(msg.message || '设备连接错误');
        } else if (msg.type === 'disconnected') {
          this.connected = false;
          this.reconnect();
        }
        return;
      }
      // 二进制视频帧
      const data = new Uint8Array(event.data);
      this.decoder.handleFrame(data);
    } catch (err) {
      console.error('[dash] message error:', err);
    }
  }

  reconnect() {
    if (this.destroyed || this.reconnectTimer) return;
    this.reconnectAttempts++;
    const delay = Math.min(1000 * this.reconnectAttempts, 5000);
    this.setStatus('重连中', '');
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  send(bytes) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(bytes);
    }
  }

  // ===== 坐标映射 =====
  toDeviceCoords(clientX, clientY) {
    const rect = this.canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return { x: 0, y: 0 };
    const scaleX = this.packer.deviceWidth / rect.width;
    const scaleY = this.packer.deviceHeight / rect.height;
    const x = Math.round((clientX - rect.left) * scaleX);
    const y = Math.round((clientY - rect.top) * scaleY);
    return {
      x: Math.max(0, Math.min(this.packer.deviceWidth - 1, x)),
      y: Math.max(0, Math.min(this.packer.deviceHeight - 1, y)),
    };
  }

  // ===== 鼠标事件 =====
  bindTouchEvents() {
    const c = this.canvas;

    c.addEventListener('mousedown', (e) => {
      e.preventDefault();
      if (e.button === 2) {
        // 右键 = 返回
        this.send(this.packer.back(KEY_UP));
        return;
      }
      this.mouseDown = true;
      const { x, y } = this.toDeviceCoords(e.clientX, e.clientY);
      this.send(this.packer.touch(ACTION_DOWN, POINTER_MOUSE, x, y, 1.0, 0x1, 0x1));
    });

    c.addEventListener('mousemove', (e) => {
      if (!this.mouseDown) return;
      e.preventDefault();
      const { x, y } = this.toDeviceCoords(e.clientX, e.clientY);
      this.send(this.packer.touch(ACTION_MOVE, POINTER_MOUSE, x, y, 1.0, 0x1, 0x1));
    });

    // mouseup 监听 window，防止鼠标移出 canvas 后无法抬起
    this._onMouseUp = (e) => {
      if (!this.mouseDown) return;
      this.mouseDown = false;
      const { x, y } = this.toDeviceCoords(e.clientX, e.clientY);
      this.send(this.packer.touch(ACTION_UP, POINTER_MOUSE, x, y, 0.0, 0, 0));
    };
    window.addEventListener('mouseup', this._onMouseUp);

    // 滚轮
    c.addEventListener('wheel', (e) => {
      e.preventDefault();
      const { x, y } = this.toDeviceCoords(e.clientX, e.clientY);
      this.send(this.packer.scroll(x, y, e.deltaX, e.deltaY));
    }, { passive: false });

    c.addEventListener('contextmenu', (e) => e.preventDefault());

    // 触摸事件（移动端）
    let touchActive = false;
    c.addEventListener('touchstart', (e) => {
      e.preventDefault();
      const t = e.touches[0];
      touchActive = true;
      const { x, y } = this.toDeviceCoords(t.clientX, t.clientY);
      this.send(this.packer.touch(ACTION_DOWN, POINTER_FINGER_BASE, x, y, t.force || 1.0));
    }, { passive: false });

    c.addEventListener('touchmove', (e) => {
      if (!touchActive) return;
      e.preventDefault();
      const t = e.touches[0];
      const { x, y } = this.toDeviceCoords(t.clientX, t.clientY);
      this.send(this.packer.touch(ACTION_MOVE, POINTER_FINGER_BASE, x, y, t.force || 1.0));
    }, { passive: false });

    c.addEventListener('touchend', (e) => {
      if (!touchActive) return;
      touchActive = false;
      const t = e.changedTouches[0];
      const { x, y } = this.toDeviceCoords(t.clientX, t.clientY);
      this.send(this.packer.touch(ACTION_UP, POINTER_FINGER_BASE, x, y, 0));
    });
  }

  // ===== 工具栏按钮 =====
  bindToolbar() {
    const phoneEl = this.cardEl.querySelector('.dash-phone');
    const tools = this.cardEl.querySelectorAll('.dash-tool');
    tools.forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        const action = btn.dataset.action;
        if (action === 'home') {
          this.send(this.packer.keyCode(KEY_DOWN, AKEYCODE.HOME, 0));
          this.send(this.packer.keyCode(KEY_UP, AKEYCODE.HOME, 0));
        } else if (action === 'recents') {
          this.send(this.packer.keyCode(KEY_DOWN, AKEYCODE.APP_SWITCH, 0));
          this.send(this.packer.keyCode(KEY_UP, AKEYCODE.APP_SWITCH, 0));
        } else if (action === 'fullscreen') {
          this.toggleFullscreen(phoneEl);
        }
      });
    });

    // 全屏退出按钮
    const fsExit = this.cardEl.querySelector('.dash-fs-exit');
    fsExit.addEventListener('click', (e) => {
      e.preventDefault();
      this.exitFullscreen(phoneEl);
    });

    // 全屏状态变化 → 切换图标 + 重算 canvas
    this._onFsChange = () => {
      const inFs = this.isFullscreen(phoneEl);
      const enter = btn.querySelector('.fs-enter') || this.cardEl.querySelector('.fs-enter');
      const exit = btn.querySelector('.fs-exit') || this.cardEl.querySelector('.fs-exit');
      this.cardEl.querySelectorAll('[data-action="fullscreen"]').forEach(b => {
        b.querySelector('.fs-enter').style.display = inFs ? 'none' : '';
        b.querySelector('.fs-exit').style.display = inFs ? '' : 'none';
      });
      setTimeout(() => this.fitCanvas(), 100);
    };
    ['fullscreenchange', 'webkitfullscreenchange'].forEach(ev =>
      document.addEventListener(ev, this._onFsChange)
    );
  }

  isFullscreen(phoneEl) {
    return !!(document.fullscreenElement || document.webkitFullscreenElement) ||
           phoneEl.classList.contains('pseudo-fs');
  }

  toggleFullscreen(phoneEl) {
    if (this.isFullscreen(phoneEl)) {
      this.exitFullscreen(phoneEl);
    } else {
      this.enterFullscreen(phoneEl);
    }
  }

  enterFullscreen(phoneEl) {
    const req = phoneEl.requestFullscreen || phoneEl.webkitRequestFullscreen;
    if (req) {
      const p = req.call(phoneEl);
      if (p && p.catch) {
        p.catch(() => {
          phoneEl.classList.add('pseudo-fs');
          this._onFsChange();
        });
      }
    } else {
      // iOS Safari 等不支持 Fullscreen API，用伪全屏
      phoneEl.classList.add('pseudo-fs');
      this._onFsChange();
    }
  }

  exitFullscreen(phoneEl) {
    if (phoneEl.classList.contains('pseudo-fs')) {
      phoneEl.classList.remove('pseudo-fs');
      this._onFsChange();
    } else {
      const ex = document.exitFullscreen || document.webkitExitFullscreen;
      if (ex) ex.call(document);
    }
  }

  // ===== 朝向检测: 横屏时旋转手机框 =====
  updateOrientation(w, h) {
    const phoneEl = this.cardEl.querySelector('.dash-phone');
    if (!phoneEl) return;
    const isLandscape = w > h;
    phoneEl.classList.toggle('landscape', isLandscape);
  }

  // ===== Canvas 自适应 =====
  // 根据容器宽高和视频宽高比，计算 canvas 最佳显示尺寸
  fitCanvas() {
    const cw = this.canvas.width;
    const ch = this.canvas.height;
    if (!cw || !ch) return;

    const availW = this.wrap.clientWidth;
    const availH = this.wrap.clientHeight;
    if (availW <= 0 || availH <= 0) return;

    const ratio = cw / ch;
    let dw = availW;
    let dh = dw / ratio;
    if (dh > availH) {
      dh = availH;
      dw = dh * ratio;
    }
    this.canvas.style.width = Math.round(dw) + 'px';
    this.canvas.style.height = Math.round(dh) + 'px';
  }

  destroy() {
    this.destroyed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    window.removeEventListener('mouseup', this._onMouseUp);
    if (this._resizeObserver) {
      this._resizeObserver.disconnect();
      this._resizeObserver = null;
    }
    if (this._onFsChange) {
      ['fullscreenchange', 'webkitfullscreenchange'].forEach(ev =>
        document.removeEventListener(ev, this._onFsChange)
      );
    }
    this.decoder.close();
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
  }
}

// ===== 页面主逻辑 =====
const clients = new Map(); // serial → DeviceClient
const grid = document.getElementById('dashboard-grid');
const countEl = document.getElementById('device-count');
let currentSerials = [];

// ===== 尺寸切换: 小/中/大 =====
// 使用 flex-wrap 布局，不需要计算列数，手机窗口自然排列自动换行
const SIZES = ['small', 'medium', 'large'];
function applySize(size) {
  grid.classList.remove(...SIZES.map(s => 'size-' + s));
  grid.classList.add('size-' + size);
  document.querySelectorAll('.size-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.size === size);
  });
  localStorage.setItem('dash_size', size);
  // ResizeObserver 会自动重算 canvas，这里额外兜底等过渡动画结束
  setTimeout(() => clients.forEach(c => c.fitCanvas()), 350);
}

// 初始化尺寸（记忆上次选择，默认 small）
const savedSize = localStorage.getItem('dash_size') || 'small';
applySize(savedSize);

document.getElementById('size-switcher').addEventListener('click', (e) => {
  const btn = e.target.closest('.size-btn');
  if (btn) applySize(btn.dataset.size);
});

function createCard(serial, device) {
  const alias = getDeviceAlias(serial);
  const name = alias || device.model || serial;
  const card = document.createElement('div');
  card.className = 'dash-card';
  card.dataset.serial = serial;
  card.innerHTML = `
    <div class="dash-phone">
      <button class="dash-fs-exit" title="退出全屏">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
      <div class="dash-screen-wrap">
        <canvas></canvas>
        <div class="dash-overlay loading">
          <div class="spinner"></div>
          <div class="overlay-text">连接中...</div>
        </div>
        <div class="dash-toolbar">
          <button class="dash-tool" data-action="home" title="返回主页">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
              <polyline points="9 22 9 12 15 12 15 22"/>
            </svg>
          </button>
          <button class="dash-tool" data-action="recents" title="切换任务">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="7" height="18" rx="1"/>
              <rect x="14" y="3" width="7" height="18" rx="1"/>
            </svg>
          </button>
          <button class="dash-tool" data-action="power" title="点亮/熄屏">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M18.36 6.64a9 9 0 1 1-12.73 0"/>
              <line x1="12" y1="2" x2="12" y2="12"/>
            </svg>
          </button>
          <button class="dash-tool" data-action="fullscreen" title="全屏">
            <svg class="fs-enter" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M8 3H5a2 2 0 0 0-2 2v3"/>
              <path d="M21 8V5a2 2 0 0 0-2-2h-3"/>
              <path d="M3 16v3a2 2 0 0 0 2 2h3"/>
              <path d="M16 21h3a2 2 0 0 0 2-2v-3"/>
            </svg>
            <svg class="fs-exit" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display:none">
              <path d="M8 3v3a2 2 0 0 1-2 2H3"/>
              <path d="M21 8h-3a2 2 0 0 1-2-2V3"/>
              <path d="M3 16h3a2 2 0 0 1 2 2v3"/>
              <path d="M16 21v-3a2 2 0 0 1 2-2h3"/>
            </svg>
          </button>
        </div>
        <div class="dash-info">
          <span class="dash-name">${name}</span>
          <span class="dash-status">连接中</span>
        </div>
      </div>
    </div>
  `;
  return card;
}

async function loadDevices() {
  try {
    const res = await fetch('/api/devices');
    const devices = await res.json();
    const serials = devices.map(d => d.serial);

    countEl.textContent = devices.length > 0
      ? `${devices.length} 台设备直播中`
      : '无设备';

    // 检测设备列表变化
    const added = serials.filter(s => !currentSerials.includes(s));
    const removed = currentSerials.filter(s => !serials.includes(s));

    if (added.length === 0 && removed.length === 0) return;

    // 移除已断开的设备
    removed.forEach(serial => {
      const client = clients.get(serial);
      if (client) {
        client.destroy();
        clients.delete(serial);
      }
      const card = grid.querySelector(`.dash-card[data-serial="${CSS.escape(serial)}"]`);
      if (card) card.remove();
    });

    // 空状态
    if (serials.length === 0) {
      grid.innerHTML = `
        <div class="dash-empty">
          <div class="big">📱</div>
          <p>未检测到在线设备</p>
          <p style="font-size:13px;margin-top:8px;color:#67707e">
            请用 <code>adb connect IP:5555</code> 连接设备
          </p>
        </div>`;
      grid.classList.add('dash-grid-empty');
      currentSerials = [];
      return;
    }

    // 移除空状态提示
    const empty = grid.querySelector('.dash-empty');
    if (empty) empty.remove();
    grid.classList.remove('dash-grid-empty');

    // 新增设备
    added.forEach(serial => {
      const device = devices.find(d => d.serial === serial);
      const card = createCard(serial, device);
      grid.appendChild(card);
      const client = new DeviceClient(serial, card);
      clients.set(serial, client);
    });

    currentSerials = serials;

    // 通知所有客户端重新计算 canvas 尺寸
    requestAnimationFrame(() => {
      clients.forEach(c => c.fitCanvas());
    });
  } catch (e) {
    countEl.textContent = '加载失败';
  }
}

// 窗口大小变化时重算所有 canvas
window.addEventListener('resize', () => {
  clients.forEach(c => c.fitCanvas());
});

// 初始化
loadDevices();
// 每 5 秒刷新设备列表
setInterval(loadDevices, 5000);

// 页面关闭时清理所有连接
window.addEventListener('beforeunload', () => {
  clients.forEach(c => c.destroy());
});
