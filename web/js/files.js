(() => {
  'use strict';

  const state = {
    path: '', query: '', type: 'all', sort: 'name', order: 'asc', items: [],
    selected: new Set(), controller: null, dialogMode: '', dialogItem: null,
    previewItem: null, uploadQueue: [], toastTimer: null, page: 1, hasMore: false,
  };
  const $ = (id) => document.getElementById(id);
  const pageParams = new URLSearchParams(location.search);
  state.serial = pageParams.get('serial') || '';

  const FOLDER_SVG = `<svg class="folder-svg-icon" width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M2.5 4.5C2.5 3.94772 2.94772 3.5 3.5 3.5H7.5C7.76522 3.5 8.01957 3.60536 8.20711 3.79289L9.91421 5.5H16.5C17.0523 5.5 17.5 5.94772 17.5 6.5V15.5C17.5 16.0523 17.0523 16.5 16.5 16.5H3.5C2.94772 16.5 2.5 16.0523 2.5 15.5V4.5Z" fill="#3888ff"/><path d="M2.5 7.5H17.5V15.5C17.5 16.0523 17.0523 16.5 16.5 16.5H3.5C2.94772 16.5 2.5 16.0523 2.5 15.5V7.5Z" fill="#165dff"/></svg>`;

  function withSerial(endpoint) { const separator = endpoint.includes('?') ? '&' : '?'; return `${endpoint}${separator}serial=${encodeURIComponent(state.serial)}`; }
  function escapeHTML(value) { return String(value ?? '').replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char])); }
  function formatBytes(value) {
    if (!value) return '—';
    const units = ['B', 'KB', 'MB', 'GB', 'TB']; let size = value; let index = 0;
    while (size >= 1024 && index < units.length - 1) { size /= 1024; index += 1; }
    return `${size >= 10 || index === 0 ? Math.round(size) : size.toFixed(1)} ${units[index]}`;
  }
  function formatTime(value) {
    if (!value) return '—';
    const date = new Date(value); return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { dateStyle: 'short', timeStyle: 'short' });
  }

  function getExt(name) {
    const idx = (name || '').lastIndexOf('.');
    return idx > 0 ? name.slice(idx + 1).toLowerCase() : '';
  }

  function getBadgeInfo(item) {
    const ext = getExt(item.name);
    const upper = ext ? ext.toUpperCase() : 'FILE';
    const text = upper.length > 4 ? upper.slice(0, 4) : upper;
    switch (ext) {
      case 'txt': case 'log':
        return { text, cls: 'badge-txt' };
      case 'json': case 'xml': case 'yaml': case 'yml': case 'sh': case 'js': case 'ts': case 'html': case 'css':
        return { text, cls: 'badge-code' };
      case 'md':
        return { text: 'MD', cls: 'badge-md' };
      case 'png': case 'jpg': case 'jpeg': case 'gif': case 'webp': case 'svg': case 'bmp': case 'ico':
        return { text, cls: 'badge-image' };
      case 'mp4': case 'mov': case 'mkv': case 'avi': case 'webm': case '3gp':
        return { text, cls: 'badge-video' };
      case 'mp3': case 'wav': case 'flac': case 'm4a': case 'aac': case 'ogg':
        return { text, cls: 'badge-audio' };
      case 'apk': case 'aab':
        return { text, cls: 'badge-apk' };
      case 'zip': case 'rar': case '7z': case 'tar': case 'gz':
        return { text, cls: 'badge-archive' };
      case 'pdf':
        return { text: 'PDF', cls: 'badge-pdf' };
      default:
        if (item.kind === 'image') return { text: text || 'IMG', cls: 'badge-image' };
        if (item.kind === 'video') return { text: text || 'VID', cls: 'badge-video' };
        if (item.kind === 'audio') return { text: text || 'AUD', cls: 'badge-audio' };
        if (item.kind === 'document') return { text: text || 'DOC', cls: 'badge-txt' };
        return { text: text || 'FILE', cls: 'badge-other' };
    }
  }

  function iconFor(item) {
    if (item.kind === 'folder') {
      return `<span class="file-folder-icon" aria-hidden="true">${FOLDER_SVG}</span>`;
    }
    const { text, cls } = getBadgeInfo(item);
    return `<span class="file-badge ${cls}" aria-hidden="true">${escapeHTML(text)}</span>`;
  }

  function getPreviewType(item) {
    if (!item || item.kind === 'folder') return null;
    const ext = getExt(item.name);
    if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico'].includes(ext) || item.kind === 'image') {
      return 'image';
    }
    if (['mp4', 'webm', 'mov', 'mkv', 'ogg'].includes(ext) || item.kind === 'video') {
      return 'video';
    }
    if (['txt', 'log', 'json', 'xml', 'md', 'yml', 'yaml', 'js', 'ts', 'html', 'css', 'sh', 'csv', 'ini', 'conf', 'properties'].includes(ext) || (item.mime && item.mime.startsWith('text/'))) {
      return 'text';
    }
    return null;
  }

  function notify(message, error = false) {
    const toast = $('toast'); toast.textContent = message; toast.className = `files-toast${error ? ' error' : ''}`; toast.hidden = false;
    clearTimeout(state.toastTimer); state.toastTimer = setTimeout(() => { toast.hidden = true; }, 4200);
  }
  function setStatus(message) { $('files-status').textContent = message; }

  async function api(endpoint, options = {}) {
    const headers = new Headers(options.headers || {});
    if (options.body && !(options.body instanceof FormData)) headers.set('Content-Type', 'application/json');
    const response = await fetch(withSerial(endpoint), { ...options, headers });
    let data = null; try { data = await response.json(); } catch (_) { /* 下载等响应没有 JSON。 */ }
    if (!response.ok) { const message = data?.error?.message || '操作失败，请重试'; const error = new Error(message); error.code = data?.error?.code; error.status = response.status; throw error; }
    return data;
  }

  async function loadFiles({ append = false } = {}) {
    if (state.controller) state.controller.abort();
    state.controller = new AbortController();
    if (!append) { state.page = 1; showState('loading'); setStatus(''); }
    const params = new URLSearchParams({ path: state.path, q: state.query, type: state.type === 'all' ? '' : state.type, sort: state.sort, order: state.order, page: String(append ? state.page + 1 : state.page), page_size: '100' });
    try {
      const data = await api(`/api/files?${params}`, { signal: state.controller.signal });
      state.page = data.page || state.page; state.hasMore = Boolean(data.has_more); state.items = append ? [...state.items, ...(data.items || [])] : (data.items || []); if (!append) state.selected.clear(); render({ ...data, items: state.items }); showState(state.items.length ? 'table' : 'empty');
      setStatus(data.total ? `${data.total} 项` : '');
    } catch (error) {
      if (error.name === 'AbortError') return;
      $('files-error-text').textContent = error.message; showState('error'); setStatus('');
    }
  }

  function showState(name) {
    $('files-loading').hidden = name !== 'loading'; $('files-error').hidden = name !== 'error'; $('files-empty').hidden = name !== 'empty'; $('files-table-wrap').hidden = name !== 'table';
  }
  function render(data) {
    $('breadcrumbs').replaceChildren(...(data.breadcrumbs || []).map((crumb, index, list) => {
      const fragment = document.createDocumentFragment();
      if (index) { const separator = document.createElement('span'); separator.textContent = ' / '; separator.className = 'breadcrumb-separator'; fragment.append(separator); }
      const button = document.createElement('button'); button.type = 'button'; button.className = `breadcrumb${index === list.length - 1 ? ' current' : ''}`; button.textContent = crumb.name; button.dataset.path = crumb.path; fragment.append(button); return fragment;
    }));
    const storage = data.storage || {}; $('storage-summary').textContent = storage.total ? `已使用 ${formatBytes(storage.used)} / ${formatBytes(storage.total)}` : '容量信息不可用';
    $('files-empty-text').textContent = state.query || state.type !== 'all' ? '没有找到匹配的文件' : '这个文件夹还是空的';
    const body = $('files-table-body'); body.replaceChildren(...state.items.map((item) => rowFor(item)));
    $('files-pagination').hidden = !state.hasMore; $('pagination-summary').textContent = state.hasMore ? `已显示 ${state.items.length} / ${data.total} 项` : '';
    updateSelectionUI();
  }
  function rowFor(item) {
    const row = document.createElement('tr'); row.dataset.path = item.path;
    const previewType = getPreviewType(item);
    const nameClass = item.kind === 'folder' ? 'folder-link' : (previewType ? 'preview-link' : '');
    const previewBtn = previewType ? `<button type="button" data-action="preview" title="预览">👁</button>` : '';
    row.innerHTML = `<td class="check-column"><input class="item-check" type="checkbox" aria-label="选择 ${escapeHTML(item.name)}"></td><td><div class="file-name-cell"><span class="file-icon">${iconFor(item)}</span><button class="file-name ${nameClass}" type="button">${escapeHTML(item.name)}</button>${state.query ? `<small class="file-path-hint">${escapeHTML(item.path)}</small>` : ''}</div></td><td data-label="类型">${escapeHTML(item.kind === 'folder' ? '文件夹' : item.mime)}</td><td data-label="大小">${item.kind === 'folder' ? '—' : formatBytes(item.size)}</td><td data-label="修改时间">${formatTime(item.modified)}</td><td class="action-column"><div class="row-actions">${previewBtn}<button type="button" data-action="download" title="下载" ${item.kind === 'folder' ? 'disabled' : ''}>↓</button><button type="button" data-action="rename" title="重命名">✎</button><button type="button" data-action="move" title="移动到">↗</button><button type="button" data-action="delete" title="删除" class="danger-icon">×</button></div></td>`;
    row.querySelector('.item-check').checked = state.selected.has(item.path);
    return row;
  }
  function updateSelectionUI() {
    const count = state.selected.size; $('selection-actions').hidden = count === 0; $('selection-count').textContent = `已选择 ${count} 项`;
    $('select-all-button').textContent = state.items.length && state.items.every((item) => state.selected.has(item.path)) ? '取消全选' : '选择全部';
  }
  function setPath(path) { state.path = path; state.query = ''; $('search-input').value = ''; loadFiles(); }

  function openActionDialog(mode, item = null) {
    state.dialogMode = mode; state.dialogItem = item;
    const title = { folder: '新建文件夹', rename: '重命名', move: '移动到' }[mode]; const label = { folder: '文件夹名称', rename: '新名称', move: '目标文件夹路径' }[mode];
    $('dialog-title').textContent = title; $('dialog-input-label').textContent = label; $('dialog-description').textContent = item ? `当前对象：${item.name}` : `将在「${state.path || '我的文件'}」中创建`;
    $('dialog-input').value = mode === 'rename' ? item.name : mode === 'move' ? state.path : ''; $('dialog-input').placeholder = mode === 'move' ? '例如：图片/2026（留空表示我的文件）' : '';
    $('action-dialog').showModal(); $('dialog-input').focus(); $('dialog-input').select();
  }
  async function performAction(value) {
    const mode = state.dialogMode; const item = state.dialogItem;
    try {
      if (mode === 'folder') await api('/api/files/folders', { method: 'POST', body: JSON.stringify({ path: state.path, name: value }) });
      if (mode === 'rename') await api('/api/files/rename', { method: 'POST', body: JSON.stringify({ path: item.path, name: value }) });
      if (mode === 'move') {
        const paths = item ? [item.path] : [...state.selected];
        for (const source of paths) await api('/api/files/move', { method: 'POST', body: JSON.stringify({ path: source, target: value, conflict: 'fail' }) });
      }
      $('action-dialog').close(); notify(mode === 'folder' ? '文件夹已创建' : '操作完成'); loadFiles();
    } catch (error) {
      if (error.code === 'conflict' && confirm('目标位置已有同名对象，是否自动保留副本？')) {
        try {
          if (mode === 'rename') await api('/api/files/rename', { method: 'POST', body: JSON.stringify({ path: item.path, name: value, conflict: 'rename' }) });
          else if (mode === 'move') { for (const source of item ? [item.path] : [...state.selected]) await api('/api/files/move', { method: 'POST', body: JSON.stringify({ path: source, target: value, conflict: 'rename' }) }); }
          $('action-dialog').close(); notify('已保留副本'); loadFiles();
        } catch (retryError) { notify(retryError.message, true); }
      } else notify(error.message, true);
    }
  }
  async function downloadItem(item) {
    try {
      const response = await fetch(withSerial(`/api/files/download?path=${encodeURIComponent(item.path)}`));
      if (!response.ok) throw new Error('下载失败，请重试');
      const blob = await response.blob(); const objectURL = URL.createObjectURL(blob); const link = document.createElement('a');
      link.href = objectURL; link.download = item.name; document.body.append(link); link.click(); link.remove(); URL.revokeObjectURL(objectURL);
    } catch (error) { notify(error.message, true); }
  }
  async function deletePaths(paths) {
    if (!paths.length || !confirm(`确定要删除 ${paths.length} 项吗？删除后可在本次会话中撤销。`)) return;
    try {
      const result = await api('/api/files/delete', { method: 'POST', body: JSON.stringify({ paths }) });
      state.selected.clear(); loadFiles();
      const toast = $('toast'); toast.innerHTML = `已移入回收站 <button id="undo-button" type="button">撤销</button>`; toast.hidden = false; toast.className = 'files-toast';
      $('undo-button').onclick = async () => { try { await api('/api/files/undo', { method: 'POST', body: JSON.stringify({ token: result.undo_token }) }); notify('已撤销删除'); loadFiles(); } catch (error) { notify(error.message, true); } };
      clearTimeout(state.toastTimer); state.toastTimer = setTimeout(() => { toast.hidden = true; }, 8000);
    } catch (error) { notify(error.message, true); }
  }

  function openPreview(item) {
    const type = getPreviewType(item);
    if (!type) return;
    state.previewItem = item;
    $('preview-filename').textContent = item.name;

    const img = $('preview-image');
    const vid = $('preview-video');
    const txt = $('preview-text');

    img.hidden = true;
    img.src = '';
    vid.hidden = true;
    vid.pause();
    vid.removeAttribute('src');
    vid.load();
    txt.hidden = true;
    txt.textContent = '';

    const previewUrl = withSerial(`/api/files/download?path=${encodeURIComponent(item.path)}&preview=1`);

    if (type === 'image') {
      img.src = previewUrl;
      img.hidden = false;
    } else if (type === 'video') {
      vid.src = previewUrl;
      vid.hidden = false;
    } else if (type === 'text') {
      txt.hidden = false;
      txt.textContent = '加载中…';
      fetch(previewUrl)
        .then(async (response) => {
          if (!response.ok) throw new Error('加载失败');
          return response.text();
        })
        .then((content) => {
          if (state.previewItem === item) {
            txt.textContent = content;
          }
        })
        .catch(() => {
          if (state.previewItem === item) {
            txt.textContent = '加载文件内容失败';
          }
        });
    }

    $('file-preview-dialog').showModal();
  }

  function closePreview() {
    const dialog = $('file-preview-dialog');
    if (dialog && dialog.open) {
      dialog.close();
    }
    state.previewItem = null;
    const vid = $('preview-video');
    if (vid) {
      vid.pause();
      vid.removeAttribute('src');
      vid.load();
    }
    if ($('preview-image')) $('preview-image').src = '';
    if ($('preview-text')) $('preview-text').textContent = '';
  }

  function startUpload(file, conflict = 'fail', existing = null) {
    const item = existing || { id: `${Date.now()}-${Math.random()}`, file, progress: 0, status: '上传中', xhr: null, conflict };
    if (!existing) state.uploadQueue.push(item); else { item.status = '上传中'; item.progress = 0; item.conflict = conflict; }
    renderUploadQueue();
    const form = new FormData(); form.append('path', state.path); form.append('conflict', conflict); form.append('file', file, file.name);
    const xhr = new XMLHttpRequest(); item.xhr = xhr; xhr.open('POST', withSerial('/api/files/upload'));
    xhr.upload.onprogress = (event) => { if (event.lengthComputable) { item.progress = Math.round(event.loaded / event.total * 100); renderUploadQueue(); } };
    xhr.onload = () => {
      let data = {}; try { data = JSON.parse(xhr.responseText); } catch (_) { /* 使用通用错误。 */ }
      if (xhr.status === 409 && conflict === 'fail' && confirm(`「${file.name}」已存在，是否保留上传副本？`)) startUpload(file, 'rename', item);
      else if (xhr.status >= 200 && xhr.status < 300) { item.progress = 100; item.status = '上传完成'; }
      else item.status = data?.error?.message || '上传失败，可重试';
      renderUploadQueue(); if (xhr.status >= 200 && xhr.status < 300) loadFiles();
    };
    xhr.onerror = () => { item.status = '网络错误，可重试'; renderUploadQueue(); };
    xhr.onabort = () => { item.status = '已取消'; renderUploadQueue(); };
    xhr.send(form);
  }
  function renderUploadQueue() {
    const active = state.uploadQueue.filter((item) => !['上传完成', '已取消'].includes(item.status)); $('upload-queue').hidden = !state.uploadQueue.length;
    const completed = state.uploadQueue.filter((item) => item.status === '上传完成').length;
    const failed = state.uploadQueue.filter((item) => item.status.includes('失败') || item.status.includes('错误')).length;
    $('upload-summary').textContent = failed ? `已完成 ${completed} 项，${failed} 项失败` : `${completed}/${state.uploadQueue.length} 项完成`;
    $('upload-list').replaceChildren(...state.uploadQueue.map((item) => { const li = document.createElement('li'); li.innerHTML = `<span class="upload-name">${escapeHTML(item.file.name)}</span><span class="upload-progress"><i style="width:${item.progress}%"></i></span><span class="upload-status">${escapeHTML(item.status)}</span>${item.status.includes('重试') ? '<button data-upload-retry type="button">重试</button>' : item.status === '上传中' ? '<button data-upload-cancel type="button">取消</button>' : ''}`; li.querySelector('[data-upload-retry]')?.addEventListener('click', () => startUpload(item.file, 'fail', item)); li.querySelector('[data-upload-cancel]')?.addEventListener('click', () => item.xhr?.abort()); return li; }));
    if (!active.length && state.uploadQueue.length) $('upload-queue').classList.add('complete'); else $('upload-queue').classList.remove('complete');
  }

  $('search-form').addEventListener('submit', (event) => { event.preventDefault(); state.query = $('search-input').value.trim(); state.path = ''; loadFiles(); });
  $('type-filter').addEventListener('change', (event) => { state.type = event.target.value; loadFiles(); });
  $('sort-select').addEventListener('change', (event) => { state.sort = event.target.value; loadFiles(); });
  $('order-button').addEventListener('click', () => { state.order = state.order === 'asc' ? 'desc' : 'asc'; $('order-button').textContent = `${$('sort-select').selectedOptions[0].textContent}${state.order === 'asc' ? '升序' : '降序'}`; loadFiles(); });
  $('breadcrumbs').addEventListener('click', (event) => { const button = event.target.closest('[data-path]'); if (button) setPath(button.dataset.path); });
  $('files-table-body').addEventListener('click', (event) => {
    const row = event.target.closest('tr');
    if (!row) return;
    const item = state.items.find((candidate) => candidate.path === row.dataset.path);
    if (!item) return;
    if (event.target.closest('.folder-link')) return setPath(item.path);
    if (event.target.closest('.preview-link')) return openPreview(item);
    const action = event.target.closest('[data-action]')?.dataset.action;
    if (action === 'preview') return openPreview(item);
    if (action === 'download') downloadItem(item);
    if (action === 'rename') openActionDialog('rename', item);
    if (action === 'move') openActionDialog('move', item);
    if (action === 'delete') deletePaths([item.path]);
  });
  $('files-table-body').addEventListener('change', (event) => { if (!event.target.classList.contains('item-check')) return; const path = event.target.closest('tr').dataset.path; if (event.target.checked) state.selected.add(path); else state.selected.delete(path); updateSelectionUI(); });
  $('select-all-button').addEventListener('click', () => { const all = state.items.length && state.items.every((item) => state.selected.has(item.path)); state.items.forEach((item) => all ? state.selected.delete(item.path) : state.selected.add(item.path)); document.querySelectorAll('.item-check').forEach((checkbox) => { checkbox.checked = state.selected.has(checkbox.closest('tr').dataset.path); }); updateSelectionUI(); });
  document.querySelectorAll('[data-batch-action]').forEach((button) => button.addEventListener('click', () => button.dataset.batchAction === 'delete' ? deletePaths([...state.selected]) : openActionDialog('move')));
  $('new-folder-button').addEventListener('click', () => openActionDialog('folder')); $('empty-upload-button').addEventListener('click', () => $('file-input').click()); $('retry-button').addEventListener('click', loadFiles);
  $('file-input').addEventListener('change', (event) => { [...event.target.files].forEach((file) => startUpload(file)); event.target.value = ''; });
  $('load-more-button').addEventListener('click', () => loadFiles({ append: true }));
  $('clear-upload-queue').addEventListener('click', () => { state.uploadQueue = state.uploadQueue.filter((item) => !['上传完成', '已取消'].includes(item.status)); renderUploadQueue(); });
  $('dialog-cancel').addEventListener('click', () => $('action-dialog').close()); $('action-form').addEventListener('submit', (event) => { event.preventDefault(); performAction($('dialog-input').value.trim()); });

  $('preview-close-btn').addEventListener('click', closePreview);
  $('preview-download-btn').addEventListener('click', () => {
    if (state.previewItem) downloadItem(state.previewItem);
  });
  $('file-preview-dialog').addEventListener('cancel', () => {
    closePreview();
  });
  $('file-preview-dialog').addEventListener('click', (event) => {
    const dialog = $('file-preview-dialog');
    const rect = dialog.getBoundingClientRect();
    const isInDialog = (
      rect.top <= event.clientY &&
      event.clientY <= rect.top + rect.height &&
      rect.left <= event.clientX &&
      event.clientX <= rect.left + rect.width
    );
    if (!isInDialog) {
      closePreview();
    }
  });

  if (state.serial) loadFiles(); else { showState('error'); $('files-error-text').textContent = '未指定手机，请从投屏页面打开文件管理'; setStatus(''); }
})();
