/**
 * template-match.js
 * 负责投屏播放器中的模板资产管理与选区直存模板。
 */
(() => {
  const params = new URLSearchParams(location.search);
  const serial = params.get('serial');
  if (!serial) return;

  // DOM Elements
  const btnManage = document.getElementById('btn-template-match') || document.getElementById('btn-template-manage');
  const btnSaveTemplate = document.getElementById('capture-save-template');

  // Save Dialog Elements
  const saveDialog = document.getElementById('template-save-dialog');
  const saveForm = document.getElementById('template-save-form');
  const saveClose = document.getElementById('template-save-dialog-close');
  const saveCancel = document.getElementById('template-save-cancel');
  const previewImg = document.getElementById('template-preview-img');
  const previewSize = document.getElementById('template-preview-size');
  const nameInput = document.getElementById('template-name-input');
  const scopeSelect = document.getElementById('template-scope-select');

  // Manage Dialog Elements
  const manageDialog = document.getElementById('template-manage-dialog');
  const manageClose = document.getElementById('template-manage-dialog-close');
  const manageRefresh = document.getElementById('template-manage-refresh');
  const listContainer = document.getElementById('template-list-container');
  const filterTabs = document.querySelectorAll('.template-tab-btn');
  const btnUpload = document.getElementById('template-btn-upload');
  const uploadInput = document.getElementById('template-upload-input');
  const btnImport = document.getElementById('template-btn-import');
  const importInput = document.getElementById('template-import-input');
  const btnExport = document.getElementById('template-btn-export');

  let currentCaptureBlob = null;
  let currentPreviewUrl = null;
  let allTemplates = [];
  let currentFilter = 'all';

  function escapeHtml(str) {
    return String(str || '').replace(/[&<>"']/g, (s) => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;',
    }[s]));
  }

  // ===== 区域截图存为模板 =====
  if (btnSaveTemplate) {
    btnSaveTemplate.addEventListener('click', async () => {
      const createFn = window.createCaptureCanvas || (typeof createCaptureCanvas === 'function' ? createCaptureCanvas : null);
      const toBlobFn = window.captureCanvasToBlob || (typeof captureCanvasToBlob === 'function' ? captureCanvasToBlob : null);

      if (!createFn || !toBlobFn) {
        alert('无法获取选区画布函数');
        return;
      }

      const capture = createFn();
      if (!capture) {
        alert('请先选定截图区域');
        return;
      }

      const blob = await toBlobFn(capture.output);
      if (!blob) {
        alert('生成选区图像失败');
        return;
      }

      currentCaptureBlob = blob;
      if (currentPreviewUrl) {
        URL.revokeObjectURL(currentPreviewUrl);
      }
      currentPreviewUrl = URL.createObjectURL(blob);
      if (previewImg) previewImg.src = currentPreviewUrl;
      if (previewSize) previewSize.textContent = `${capture.sw} × ${capture.sh} px`;

      if (nameInput) {
        nameInput.value = '';
        setTimeout(() => nameInput.focus(), 50);
      }
      if (scopeSelect) scopeSelect.value = 'device';

      if (saveDialog && typeof saveDialog.showModal === 'function') {
        saveDialog.showModal();
      }
    });
  }

  function closeSaveDialog() {
    if (saveDialog) saveDialog.close();
    if (currentPreviewUrl) {
      URL.revokeObjectURL(currentPreviewUrl);
      currentPreviewUrl = null;
    }
    currentCaptureBlob = null;
  }

  if (saveClose) saveClose.addEventListener('click', closeSaveDialog);
  if (saveCancel) saveCancel.addEventListener('click', closeSaveDialog);

  if (saveForm) {
    saveForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!currentCaptureBlob) {
        alert('选区图片丢失，请重新选择');
        return;
      }
      const name = nameInput ? nameInput.value.trim() : '';
      if (!name) {
        alert('请输入模板名称');
        return;
      }

      const scope = scopeSelect ? scopeSelect.value : 'device';
      const targetSerial = scope === 'global' ? 'global' : serial;
      const formData = new FormData();
      formData.append('image', currentCaptureBlob, 'template.png');
      formData.append('name', name);
      formData.append('serial', targetSerial);
      const screenCanvas = document.getElementById('screen');
      if (screenCanvas && screenCanvas.width && screenCanvas.height) {
        formData.append('scene_width', String(screenCanvas.width));
        formData.append('scene_height', String(screenCanvas.height));
      }

      const submitBtn = document.getElementById('template-save-confirm');
      if (submitBtn) submitBtn.disabled = true;

      try {
        const res = await fetch('/api/templates', {
          method: 'POST',
          body: formData,
        });
        if (!res.ok) {
          const errData = await res.json().catch(() => ({}));
          throw new Error(errData.message || `HTTP ${res.status}`);
        }

        closeSaveDialog();

        // 成功保存后关闭选区
        const closeFn = window.closeCapture || (typeof closeCapture === 'function' ? closeCapture : null);
        if (closeFn) closeFn();

        loadTemplates();
      } catch (err) {
        console.error('保存模板失败:', err);
        alert(`保存模板失败: ${err.message}`);
      } finally {
        if (submitBtn) submitBtn.disabled = false;
      }
    });
  }

  // ===== 模板管理弹窗 =====
  function getFilteredTemplates() {
    if (currentFilter === 'device') {
      return allTemplates.filter((t) => t.serial === serial);
    }
    if (currentFilter === 'global') {
      return allTemplates.filter((t) => t.serial === 'global');
    }
    return allTemplates;
  }

  function renderTemplateCards() {
    if (!listContainer) return;
    const list = getFilteredTemplates();
    if (!list || list.length === 0) {
      listContainer.innerHTML = '<div class="template-empty-hint">暂无模板。可通过截图选区点击“存为模板”添加。</div>';
      return;
    }

    listContainer.innerHTML = '';
    const grid = document.createElement('div');
    grid.className = 'template-cards-grid';

    list.forEach((t) => {
      const card = document.createElement('div');
      card.className = `template-card ${t.enabled ? '' : 'disabled'}`;
      card.dataset.id = t.id;

      const isGlobal = t.serial === 'global';
      const scopeBtnHtml = isGlobal
        ? `<button type="button" class="template-card-scope-btn" data-id="${t.id}" data-action="to-device" title="转为当前设备模板">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="5" y="2" width="14" height="20" rx="2" ry="2"></rect>
              <line x1="12" y1="18" x2="12.01" y2="18"></line>
            </svg>
          </button>`
        : `<button type="button" class="template-card-scope-btn" data-id="${t.id}" data-action="to-global" title="转为全局模板">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="2" y1="12" x2="22" y2="12"></line>
              <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path>
            </svg>
          </button>`;

      card.innerHTML = `
        <div class="template-card-thumb-wrap">
          <img src="/api/templates/${encodeURIComponent(t.id)}/image?_t=${t.updated_at || Date.now()}" alt="${escapeHtml(t.name)}" class="template-card-thumb" loading="lazy" />
          <span class="template-card-badge ${isGlobal ? 'global' : 'device'}">${isGlobal ? '全局' : '设备'}</span>
        </div>
        <div class="template-card-main">
          <div class="template-card-title-row">
            <span class="template-card-title" title="${escapeHtml(t.name)}">${escapeHtml(t.name)}</span>
            <span class="template-card-dim">${t.width || 0}×${t.height || 0} · ${t.image_count || 1} 图</span>
          </div>
          <div class="template-card-controls">
            <div class="template-card-actions">
              <div class="template-card-btns">
                ${scopeBtnHtml}
                <button type="button" class="template-card-add-image-btn" data-id="${t.id}" title="添加参考图" aria-label="添加参考图"><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="13" height="13" rx="2"/><circle cx="7.5" cy="8.5" r="1"/><path d="m4 15 4-4 3 3 2-2 3 3"/><path d="M19 12v7M15.5 15.5h7"/></svg></button>
                <a href="/api/templates/${encodeURIComponent(t.id)}/image?download=1" class="template-card-dl-btn" title="下载原图" download="${escapeHtml(t.name)}.png">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"></path>
                  </svg>
                </a>
                <button type="button" class="template-card-del-btn" data-id="${t.id}" data-name="${escapeHtml(t.name)}" title="删除模板">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="3 6 5 6 21 6"></polyline>
                    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>
      `;

      grid.appendChild(card);
    });

    listContainer.appendChild(grid);

    // 绑定卡片内事件
    grid.querySelectorAll('.template-card-scope-btn').forEach((btn) => {
      const id = btn.dataset.id;
      const action = btn.dataset.action;
      btn.addEventListener('click', async () => {
        const newSerial = action === 'to-global' ? 'global' : serial;
        const success = await updateTemplate(id, { serial: newSerial });
        if (success) {
          loadTemplates();
        }
      });
    });

    grid.querySelectorAll('.template-card-add-image-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        const input = document.createElement('input'); input.type = 'file'; input.accept = 'image/png,image/jpeg';
        input.onchange = async () => { const file = input.files && input.files[0]; if (!file) return; const body = new FormData(); body.append('image', file); btn.disabled = true; try { const res = await fetch(`/api/templates/${encodeURIComponent(btn.dataset.id)}/images`, { method:'POST', body }); if (!res.ok) throw new Error(); await loadTemplates(); } catch (_) { alert('添加参考图失败，请重试'); } finally { btn.disabled = false; } };
        input.click();
      });
    });

    grid.querySelectorAll('.template-card-del-btn').forEach((btn) => {
      const id = btn.dataset.id;
      const name = btn.dataset.name;
      btn.addEventListener('click', async () => {
        if (!confirm(`确定要删除模板 “${name}” 吗？`)) return;
        await deleteTemplate(id);
      });
    });
  }

  async function loadTemplates() {
    if (!listContainer) return;
    listContainer.innerHTML = '<div class="template-empty-hint">加载中...</div>';
    try {
      const res = await fetch(`/api/templates?serial=${encodeURIComponent(serial)}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      allTemplates = Array.isArray(data.templates) ? data.templates : [];
      renderTemplateCards();
    } catch (err) {
      console.error('加载模板列表失败:', err);
      listContainer.innerHTML = `<div class="template-empty-hint error">加载失败: ${escapeHtml(err.message)}</div>`;
    }
  }

  async function updateTemplate(id, patch) {
    try {
      const res = await fetch(`/api/templates/${encodeURIComponent(id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(patch),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const updated = await res.json();
      const idx = allTemplates.findIndex((t) => t.id === id);
      if (idx >= 0) allTemplates[idx] = updated;
      return true;
    } catch (err) {
      console.error('更新模板失败:', err);
      alert('更新模板失败，请重试');
      return false;
    }
  }

  async function deleteTemplate(id) {
    try {
      const res = await fetch(`/api/templates/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      allTemplates = allTemplates.filter((t) => t.id !== id);
      renderTemplateCards();
    } catch (err) {
      console.error('删除模板失败:', err);
      alert('删除模板失败，请重试');
    }
  }

  if (btnManage) {
    btnManage.addEventListener('click', () => {
      const moreMenu = document.getElementById('more-menu');
      if (moreMenu) moreMenu.hidden = true;

      if (manageDialog) {
        if (window.RightPanel) { manageDialog.close?.(); manageDialog.hidden = true; window.RightPanel.open(manageDialog); } else manageDialog.showModal?.();
        loadTemplates();
      }
    });
  }

  if (manageClose) {
    manageClose.addEventListener('click', () => {
      if (manageDialog) { if (window.RightPanel) window.RightPanel.close(manageDialog); else manageDialog.close(); }
    });
  }

  if (manageRefresh) {
    manageRefresh.addEventListener('click', () => {
      loadTemplates();
    });
  }

  // 单张上传 PNG 模板
  if (btnUpload && uploadInput) {
    btnUpload.addEventListener('click', () => {
      uploadInput.click();
    });

    uploadInput.addEventListener('change', async () => {
      const file = uploadInput.files?.[0];
      if (!file) return;

      const formData = new FormData();
      formData.append('image', file);
      const targetSerial = currentFilter === 'global' ? 'global' : serial;
      formData.append('serial', targetSerial);
      formData.append('threshold', '0.88');
      const name = file.name.replace(/\.[^/.]+$/, '');
      if (name) {
        formData.append('name', name);
      }

      try {
        const res = await fetch('/api/templates', {
          method: 'POST',
          body: formData,
        });
        if (!res.ok) {
          const errData = await res.json().catch(() => ({}));
          throw new Error(errData.message || `HTTP ${res.status}`);
        }
        uploadInput.value = '';
        loadTemplates();
      } catch (err) {
        console.error('上传模板失败:', err);
        alert(`上传模板失败: ${err.message}`);
        uploadInput.value = '';
      }
    });
  }

  // 导入 ZIP
  if (btnImport && importInput) {
    btnImport.addEventListener('click', () => {
      importInput.click();
    });

    importInput.addEventListener('change', async () => {
      const file = importInput.files?.[0];
      if (!file) return;

      const formData = new FormData();
      formData.append('file', file);
      const targetSerial = currentFilter === 'global' ? 'global' : serial;
      formData.append('serial', targetSerial);

      try {
        const res = await fetch('/api/templates/import', {
          method: 'POST',
          body: formData,
        });
        if (!res.ok) {
          const errData = await res.json().catch(() => ({}));
          throw new Error(errData.message || `HTTP ${res.status}`);
        }
        const data = await res.json();
        alert(`成功导入 ${data.imported || 0} 个模板`);
        importInput.value = '';
        loadTemplates();
      } catch (err) {
        console.error('导入 ZIP 失败:', err);
        alert(`导入 ZIP 失败: ${err.message}`);
        importInput.value = '';
      }
    });
  }

  // 导出 ZIP
  if (btnExport) {
    btnExport.addEventListener('click', () => {
      const scope = currentFilter;
      const exportUrl = `/api/templates/export?serial=${encodeURIComponent(serial)}&scope=${encodeURIComponent(scope)}`;
      window.open(exportUrl, '_blank');
    });
  }

  filterTabs.forEach((tab) => {
    tab.addEventListener('click', () => {
      filterTabs.forEach((t) => t.classList.remove('active'));
      tab.classList.add('active');
      currentFilter = tab.dataset.filter || 'all';
      renderTemplateCards();
    });
  });

  // 初始化
  initMatchingStatus();
})();
