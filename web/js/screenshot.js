(() => {
  const serial = new URLSearchParams(location.search).get('serial');
  const tools = document.querySelector('.player-toolbar .tools');
  const toolbar = document.querySelector('.player-toolbar');
  const moreButton = document.getElementById('btn-more');
  const canvas = document.getElementById('screen');
  if (!tools || !toolbar || !moreButton || !canvas || !serial) return;

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

  const btn = document.createElement('button');
  btn.id = 'btn-screenshot';
  btn.type = 'button';
  btn.title = '截图';
  btn.setAttribute('aria-label', '截图');
  btn.innerHTML = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg><span>截图</span>';
  btn.className = 'screenshot-btn';
  btn.setAttribute('aria-haspopup', 'dialog');
  tools.insertBefore(btn, moreButton);

  const panel = document.createElement('div');
  panel.id = 'screenshot-panel';
  panel.className = 'recording-panel screenshot-panel';
  panel.hidden = true;
  panel.setAttribute('role', 'dialog');
  panel.setAttribute('aria-label', '截图记录');
  panel.innerHTML = `
    <section class="recording-history" style="margin-top:0;padding-top:0;border-top:0">
      <h2>截图记录</h2>
      <p class="recording-history-empty">暂无截图，点击工具栏“截图”保存当前画面</p>
      <ul></ul>
    </section>
    <div class="recording-panel-actions"><button id="screenshot-close" type="button">关闭</button></div>`;
  toolbar.append(panel);
  const list = panel.querySelector('ul');
  const empty = panel.querySelector('.recording-history-empty');
  panel.querySelector('#screenshot-close').onclick = () => { panel.hidden = true; };

  const modal = document.createElement('div');
  modal.className = 'screenshot-modal';
  modal.hidden = true;
  modal.innerHTML = `<div class="screenshot-modal-body" role="dialog" aria-label="查看截图"><img alt="截图大图"><div class="screenshot-modal-actions"><button type="button">关闭</button></div></div>`;
  document.body.append(modal);
  const modalImg = modal.querySelector('img');
  modal.querySelector('button').onclick = () => { modal.hidden = true; modalImg.removeAttribute('src'); };
  modal.addEventListener('click', (e) => { if (e.target === modal) modal.hidden = true; });

  const fmtTime = (s) => {
    try { return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(s)); }
    catch (_) { return s || ''; }
  };
  const viewUrl = (id) => `/api/screenshots/${encodeURIComponent(id)}?serial=${encodeURIComponent(serial)}`;

  const loadList = async () => {
    try {
      const res = await fetch(`/api/screenshots?serial=${encodeURIComponent(serial)}`);
      if (res.status === 404 || res.status === 501) { empty.hidden = false; empty.textContent = '截图接口暂未就绪'; return; }
      if (!res.ok) throw Error();
      const { screenshots = [] } = await res.json();
      list.replaceChildren();
      empty.hidden = screenshots.length > 0;
      screenshots.forEach((s) => {
        const li = document.createElement('li');
        const thumb = document.createElement('img');
        thumb.className = 'screenshot-thumb';
        thumb.alt = '截图缩略图';
        thumb.loading = 'lazy';
        thumb.src = viewUrl(s.screenshot_id);
        const details = document.createElement('div');
        details.innerHTML = `<strong>${fmtTime(s.created_at)}</strong><span>${s.width || '?'} × ${s.height || '?'}</span>`;
        const actions = document.createElement('div');
        actions.className = 'recording-history-actions';
        const view = document.createElement('button');
        view.type = 'button'; view.textContent = '查看';
        view.onclick = () => { modalImg.src = viewUrl(s.screenshot_id); modal.hidden = false; };
        const dl = document.createElement('a');
        dl.textContent = '下载';
        dl.href = `${viewUrl(s.screenshot_id)}&download=1`;
        const del = document.createElement('button');
        del.type = 'button'; del.textContent = '删除';
        del.onclick = async () => {
          if (!confirm('确定删除这张截图吗？')) return;
          del.disabled = true;
          try {
            const r = await fetch(viewUrl(s.screenshot_id), { method: 'DELETE' });
            if (!r.ok && r.status !== 204) throw Error();
            toast('已删除截图');
            await loadList();
          } catch (_) { toast('删除截图失败，请重试'); }
          finally { del.disabled = false; }
        };
        actions.append(view, dl, del);
        li.append(thumb, details, actions);
        list.append(li);
      });
    } catch (_) { empty.hidden = false; empty.textContent = '暂时无法获取截图记录'; }
  };

  btn.addEventListener('click', async () => {
    // 点击即截图上传，成功后展开面板展示记录；面板已打开时同样先上传再刷新
    if (!canvas.width || !canvas.height) { toast('画面尚未准备好，暂时无法截图'); return; }
    btn.disabled = true;
    try {
      const blob = await new Promise((resolve) => canvas.toBlob(resolve, 'image/png'));
      if (!blob) { toast('截图生成失败，请重试'); return; }
      const res = await fetch(`/api/screenshots?serial=${encodeURIComponent(serial)}`, { method: 'POST', body: blob });
      if (res.status === 404 || res.status === 501) { toast('截图接口暂未就绪，请稍后重试'); return; }
      if (!res.ok) throw Error();
      toast('截图已保存');
    } catch (_) { toast('截图上传失败，请重试'); }
    finally { btn.disabled = false; }
    panel.hidden = false;
    window.placeToolbarPopover(panel, btn);
    await loadList();
  });
})();
