// touch-visualizer.js — 投屏上的触摸可视化（类似安卓"显示触摸"）。
// 服务端把 tap/swipe/原始触摸事件推到 /api/vision/results 通道，
// player.html 的 onmessage 里调用 TouchViz.handle(msg) 分发到本模块。
(function() {
  const toggleBtn = document.getElementById('btn-toggle-touch');
  const overlay = document.getElementById('touch-overlay');
  if (!overlay) return;
  const ctx = overlay.getContext('2d');

  let enabled = localStorage.getItem('show-touch-overlay') !== 'false';
  if (toggleBtn) {
    toggleBtn.checked = enabled;
    toggleBtn.addEventListener('change', () => {
      enabled = toggleBtn.checked;
      localStorage.setItem('show-touch-overlay', enabled);
      if (!enabled) reset();
    });
  }

  // ripple: {x,y,born} 单击波纹；swipe: {x,y,x2,y2,born} 拖动轨迹线
  const effects = [];
  // trail: 手指按住拖动的实时轨迹点，按时间淡出
  const trail = [];
  let rafId = 0;

  const RIPPLE_MS = 600;
  const SWIPE_MS = 900;
  const TRAIL_MS = 700;

  function px(msg) {
    return {
      x: msg.x * overlay.width,
      y: msg.y * overlay.height,
      x2: (msg.x2 || 0) * overlay.width,
      y2: (msg.y2 || 0) * overlay.height,
    };
  }

  function handle(msg) {
    if (!enabled || !msg || !overlay.width || !overlay.height) return;
    // 会话过滤由服务端订阅（session_id 参数）完成，这里无需重复判断。
    const p = px(msg);
    const now = performance.now();
    if (msg.kind === 'tap') {
      effects.push({ type: 'ripple', x: p.x, y: p.y, born: now });
    } else if (msg.kind === 'swipe') {
      effects.push({ type: 'swipe', x: p.x, y: p.y, x2: p.x2, y2: p.y2, born: now });
    } else if (msg.kind === 'down') {
      trail.push({ x: p.x, y: p.y, t: now });
    } else if (msg.kind === 'move') {
      trail.push({ x: p.x, y: p.y, t: now });
      if (trail.length > 200) trail.splice(0, trail.length - 200);
    } else if (msg.kind === 'up') {
      effects.push({ type: 'ripple', x: p.x, y: p.y, born: now });
    }
    startLoop();
  }

  function drawFinger(ctx2, x, y, alpha, scale) {
    const r = Math.max(10, overlay.width / 60) * scale;
    ctx2.beginPath();
    ctx2.arc(x, y, r, 0, Math.PI * 2);
    ctx2.fillStyle = `rgba(255, 255, 255, ${0.45 * alpha})`;
    ctx2.fill();
    ctx2.lineWidth = Math.max(2, overlay.width / 400);
    ctx2.strokeStyle = `rgba(0, 150, 255, ${0.9 * alpha})`;
    ctx2.stroke();
  }

  function render() {
    const now = performance.now();
    ctx.clearRect(0, 0, overlay.width, overlay.height);

    // 拖动轨迹：相邻点连线，按年龄淡出
    while (trail.length && now - trail[0].t > TRAIL_MS) trail.shift();
    if (trail.length) {
      ctx.lineWidth = Math.max(3, overlay.width / 300);
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';
      for (let i = 1; i < trail.length; i++) {
        const a = trail[i - 1];
        const b = trail[i];
        const age = (now - b.t) / TRAIL_MS;
        const alpha = 1 - age;
        ctx.strokeStyle = `rgba(0, 150, 255, ${0.7 * alpha})`;
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.stroke();
      }
      const last = trail[trail.length - 1];
      drawFinger(ctx, last.x, last.y, 1, 1);
    }

    for (let i = effects.length - 1; i >= 0; i--) {
      const e = effects[i];
      if (e.type === 'ripple') {
        const t = (now - e.born) / RIPPLE_MS;
        if (t >= 1) { effects.splice(i, 1); continue; }
        // 内圈指印保持，外圈波纹扩散淡出
        drawFinger(ctx, e.x, e.y, 1 - t, 1);
        ctx.beginPath();
        ctx.arc(e.x, e.y, Math.max(10, overlay.width / 60) + t * overlay.width / 25, 0, Math.PI * 2);
        ctx.strokeStyle = `rgba(0, 150, 255, ${0.8 * (1 - t)})`;
        ctx.lineWidth = Math.max(2, overlay.width / 400);
        ctx.stroke();
      } else if (e.type === 'swipe') {
        const t = (now - e.born) / SWIPE_MS;
        if (t >= 1) { effects.splice(i, 1); continue; }
        // 轨迹线随时间推进绘制，末端小圆点，之后整体淡出
        const head = Math.min(1, t * 1.4);
        const fade = t < 0.7 ? 1 : 1 - (t - 0.7) / 0.3;
        ctx.strokeStyle = `rgba(0, 150, 255, ${0.85 * fade})`;
        ctx.lineWidth = Math.max(4, overlay.width / 250);
        ctx.lineCap = 'round';
        ctx.beginPath();
        ctx.moveTo(e.x, e.y);
        ctx.lineTo(e.x + (e.x2 - e.x) * head, e.y + (e.y2 - e.y) * head);
        ctx.stroke();
        if (head < 1) {
          drawFinger(ctx, e.x + (e.x2 - e.x) * head, e.y + (e.y2 - e.y) * head, 1, 1);
        } else {
          drawFinger(ctx, e.x2, e.y2, fade, 1);
        }
      }
    }

    if (effects.length || trail.length) {
      rafId = requestAnimationFrame(render);
    } else {
      rafId = 0;
      ctx.clearRect(0, 0, overlay.width, overlay.height);
    }
  }

  function startLoop() {
    if (!rafId) rafId = requestAnimationFrame(render);
  }

  function reset() {
    effects.length = 0;
    trail.length = 0;
    if (rafId) { cancelAnimationFrame(rafId); rafId = 0; }
    ctx.clearRect(0, 0, overlay.width, overlay.height);
  }

  window.TouchViz = { handle, reset };
})();
