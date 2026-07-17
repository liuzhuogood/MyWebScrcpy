// scripts.js — 脚本管理页逻辑
// 分类管理 + 脚本列表 + 在线代码编辑器 (CodeMirror 6，离线降级 textarea)

// ===== 状态 =====
let categories = [];
let currentCategory = '';
let currentScriptName = '';
let pristineScript = null; // 上次保存时的快照
let currentScript = null;  // 当前编辑值
let editorView = null;     // CodeMirror 或 textarea 引用
let cmLoaded = false;      // CodeMirror 是否成功加载
let saving = false;        // 防止重复保存

// ===== DOM 引用 =====
const catList = document.getElementById('cat-list');
const btnAddCat = document.getElementById('btn-add-cat');
const scriptListView = document.getElementById('script-list-view');
const editorViewEl = document.getElementById('editor-view');
const currentCatTitle = document.getElementById('current-cat-title');
const scriptList = document.getElementById('script-list');
const btnAddScript = document.getElementById('btn-add-script');
const btnBackList = document.getElementById('btn-back-list');
const scriptNameInput = document.getElementById('script-name');
const scriptDescInput = document.getElementById('script-desc');
const scriptParamsInput = document.getElementById('script-params');
const editorContainer = document.getElementById('editor-container');
const btnSave = document.getElementById('btn-save');
const btnDeleteScript = document.getElementById('btn-delete-script');
const editorStatus = document.getElementById('editor-status');

// ===== 分类管理 =====

// 加载分类列表
async function loadCategories() {
  try {
    const res = await fetch('/api/scripts/categories');
    const data = await res.json();
    categories = data.categories || [];
    renderCategories();
  } catch (e) {
    catList.innerHTML = '<li class="empty-item">加载失败</li>';
  }
}

// 渲染分类列表
function renderCategories() {
  if (categories.length === 0) {
    catList.innerHTML = '<li class="empty-item">暂无分类，点击 + 新建</li>';
    return;
  }
  catList.innerHTML = categories.map(cat => {
    const active = cat === currentCategory ? ' class="active"' : '';
    return `<li${active} data-cat="${escapeHtml(cat)}">
      <span class="cat-name" onclick="selectCategory('${escapeHtml(cat)}')">${escapeHtml(cat)}</span>
      <span class="cat-actions">
        <button class="cat-action" onclick="renameCategory('${escapeHtml(cat)}')" title="重命名">✎</button>
        <button class="cat-action danger" onclick="deleteCategory('${escapeHtml(cat)}')" title="删除">✕</button>
      </span>
    </li>`;
  }).join('');
}

// 选择分类
function selectCategory(cat) {
  currentCategory = cat;
  renderCategories();
  loadScripts(cat);
  document.getElementById('current-cat-title').textContent = cat;
  btnAddScript.style.display = 'inline-flex';
  // 隐藏编辑器
  editorViewEl.style.display = 'none';
  scriptListView.style.display = '';
}

// 新建分类
btnAddCat.addEventListener('click', () => {
  const name = prompt('请输入新分类名称：');
  if (!name || !name.trim()) return;
  const trimmed = name.trim();
  fetch('/api/scripts/categories', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({name: trimmed})
  }).then(async res => {
    const data = await res.json();
    if (!res.ok) { alert(data.error || '新建分类失败'); return; }
    loadCategories();
    selectCategory(trimmed);
  }).catch(e => alert('请求失败'));
});

// 重命名分类
function renameCategory(oldName) {
  const newName = prompt('请输入新分类名称：', oldName);
  if (!newName || !newName.trim() || newName.trim() === oldName) return;
  const trimmed = newName.trim();
  fetch('/api/scripts/categories/' + encodeURIComponent(oldName), {
    method: 'PATCH',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({name: trimmed})
  }).then(async res => {
    const data = await res.json();
    if (!res.ok) { alert(data.error || '重命名失败'); return; }
    if (currentCategory === oldName) currentCategory = trimmed;
    loadCategories();
    if (currentCategory === trimmed) selectCategory(trimmed);
  }).catch(e => alert('请求失败'));
}

// 删除分类
function deleteCategory(name) {
  if (!confirm(`确定删除分类「${name}」及其下所有脚本？此操作不可恢复。`)) return;
  fetch('/api/scripts/categories/' + encodeURIComponent(name) + '?confirm=true', {
    method: 'DELETE'
  }).then(async res => {
    const data = await res.json();
    if (!res.ok) { alert(data.error || '删除失败'); return; }
    if (currentCategory === name) {
      currentCategory = '';
      scriptListView.style.display = '';
      editorViewEl.style.display = 'none';
      currentCatTitle.textContent = '请选择一个分类';
      btnAddScript.style.display = 'none';
    }
    loadCategories();
  }).catch(e => alert('请求失败'));
}

// ===== 脚本列表 =====

// 加载脚本列表
async function loadScripts(category) {
  try {
    const res = await fetch('/api/scripts?category=' + encodeURIComponent(category));
    const data = await res.json();
    const scripts = data.scripts || [];
    renderScripts(scripts);
  } catch (e) {
    scriptList.innerHTML = '<li class="empty-item">加载失败</li>';
  }
}

function renderScripts(scripts) {
  if (scripts.length === 0) {
    scriptList.innerHTML = '<li class="empty-item">该分类下暂无脚本</li>';
    return;
  }
  scriptList.innerHTML = scripts.map(s => {
    const active = s.name === currentScriptName ? ' class="active"' : '';
    return `<li${active} data-script="${escapeHtml(s.name)}" onclick="selectScript('${escapeHtml(s.name)}')">
      <span class="script-name">${escapeHtml(s.name)}</span>
      <span class="script-meta">${s.description ? escapeHtml(s.description) : '无描述'}</span>
      <button class="script-del" onclick="event.stopPropagation(); deleteScript('${escapeHtml(s.name)}')" title="删除">✕</button>
    </li>`;
  }).join('');
}

// 选择脚本
function selectScript(name) {
  currentScriptName = name;
  loadScriptDetail(currentCategory, name);
}

// 新建脚本
btnAddScript.addEventListener('click', () => {
  const name = prompt('请输入新脚本名称（不含 .py）：');
  if (!name || !name.trim()) return;
  const trimmed = name.trim();
  const template = 'import subprocess\n\n\ndef main():\n    """在此实现你的脚本逻辑"""\n    pass\n';
  fetch('/api/scripts/' + encodeURIComponent(currentCategory), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({name: trimmed, description: '', params: '', content: template})
  }).then(async res => {
    const data = await res.json();
    if (!res.ok) { alert(data.error || '新建失败'); return; }
    loadScripts(currentCategory);
    selectScript(trimmed);
  }).catch(e => alert('请求失败'));
});

// 删除脚本
function deleteScript(name) {
  if (!confirm(`确定删除脚本「${name}」？`)) return;
  fetch('/api/scripts/' + encodeURIComponent(currentCategory) + '/' + encodeURIComponent(name), {
    method: 'DELETE'
  }).then(async res => {
    const data = await res.json();
    if (!res.ok) { alert(data.error || '删除失败'); return; }
    if (currentScriptName === name) {
      currentScriptName = '';
      editorViewEl.style.display = 'none';
      scriptListView.style.display = '';
    }
    loadScripts(currentCategory);
  }).catch(e => alert('请求失败'));
}

// ===== 编辑器 =====

// 加载脚本详情到编辑器
async function loadScriptDetail(category, name) {
  try {
    const res = await fetch('/api/scripts/' + encodeURIComponent(category) + '/' + encodeURIComponent(name));
    if (!res.ok) { alert('读取脚本失败'); return; }
    const detail = await res.json();
    scriptListView.style.display = 'none';
    editorViewEl.style.display = '';

    scriptNameInput.value = detail.name || '';
    scriptDescInput.value = detail.description || '';
    scriptParamsInput.value = detail.params || '';

    // 保存干净状态
    pristineScript = {
      name: detail.name,
      description: detail.description,
      params: detail.params,
      content: detail.content || ''
    };
    currentScript = {...pristineScript};

    // 更新脚本列表高亮
    renderScriptListHighlight();

    // 设置编辑器内容
    setEditorContent(detail.content || '');
    updateSaveButton();
  } catch (e) {
    alert('请求失败');
  }
}

function renderScriptListHighlight() {
  const items = scriptList.querySelectorAll('li');
  items.forEach(li => {
    li.classList.toggle('active', li.dataset.script === currentScriptName);
  });
}

// 编辑器初始化
async function initEditor() {
  try {
    const cmMod = await import('https://cdn.jsdelivr.net/npm/codemirror@6.0.1/+esm');
    const pyMod = await import('https://cdn.jsdelivr.net/npm/@codemirror/lang-python@6.0.0/+esm');
    const {EditorView, basicSetup} = cmMod;
    const {python} = pyMod;

    cmLoaded = true;

    // 创建 CodeMirror 编辑器
    function createEditor(doc, onChange) {
      const state = EditorView.createState({
        doc: doc || '',
        extensions: [
          basicSetup,
          python(),
          EditorView.updateListener.of(update => {
            if (update.docChanged) {
              onChange(update.state.doc.toString());
            }
          }),
          EditorView.theme({
            '&': {height: '100%'},
            '.cm-scroller': {overflow: 'auto'}
          })
        ]
      });
      return new EditorView({state, parent: editorContainer});
    }

    window.__cmCreateEditor = createEditor;
    console.log('[scripts] CodeMirror 6 加载成功');
  } catch (e) {
    console.warn('[scripts] CodeMirror 加载失败，降级为 textarea:', e);
    cmLoaded = false;

    // textarea 降级
    function createEditor(doc, onChange) {
      const ta = document.createElement('textarea');
      ta.className = 'cm-fallback';
      ta.value = doc || '';
      editorContainer.appendChild(ta);

      ta.addEventListener('input', () => {
        onChange(ta.value);
      });

      return {
        getValue: () => ta.value,
        setValue: (v) => { ta.value = v; },
        destroy: () => { ta.remove(); }
      };
    }

    window.__cmCreateEditor = createEditor;
  }
}

// 设置编辑器内容
function setEditorContent(content) {
  if (editorView) {
    editorView.destroy();
    editorView = null;
  }
  editorContainer.innerHTML = '';

  editorView = window.__cmCreateEditor(content, onEditorChange);
}

function onEditorChange(newContent) {
  if (!currentScript) return;
  currentScript.content = newContent;
  updateSaveButton();
}

// ===== 表单变化 =====
scriptNameInput.addEventListener('input', () => {
  if (!currentScript) return;
  currentScript.name = scriptNameInput.value;
  updateSaveButton();
});
scriptDescInput.addEventListener('input', () => {
  if (!currentScript) return;
  currentScript.description = scriptDescInput.value;
  updateSaveButton();
});
scriptParamsInput.addEventListener('input', () => {
  if (!currentScript) return;
  currentScript.params = scriptParamsInput.value;
  updateSaveButton();
});

// ===== 保存按钮状态 =====
function isDirty() {
  if (!pristineScript || !currentScript) return false;
  return currentScript.name !== pristineScript.name ||
         currentScript.description !== pristineScript.description ||
         currentScript.params !== pristineScript.params ||
         currentScript.content !== pristineScript.content;
}

function updateSaveButton() {
  const dirty = isDirty();
  btnSave.disabled = !dirty || saving;
  editorStatus.textContent = dirty ? '有未保存的更改' : '';
}

// ===== 保存 =====
btnSave.addEventListener('click', async () => {
  if (saving || !isDirty()) return;
  saving = true;
  btnSave.disabled = true;
  btnSave.textContent = '保存中...';

  try {
    const body = {
      name: currentScript.name,
      description: currentScript.description,
      params: currentScript.params,
      content: currentScript.content,
      updated: pristineScript ? pristineScript.updated : undefined
    };

    const res = await fetch('/api/scripts/' + encodeURIComponent(currentCategory) + '/' + encodeURIComponent(currentScriptName), {
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body)
    });

    const data = await res.json();

    if (res.status === 409) {
      alert('保存冲突：该脚本已被其他操作修改，请刷新后重试');
      return;
    }

    if (!res.ok) {
      alert(data.error || '保存失败');
      return;
    }

    // 更新干净状态
    pristineScript = {...currentScript, updated: data.script ? data.script.updated : undefined};

    // 如果改名了，更新当前脚本名和列表
    if (currentScript.name !== currentScriptName) {
      const oldName = currentScriptName;
      currentScriptName = currentScript.name;
      loadScripts(currentCategory);
    } else {
      // 刷新脚本列表以更新元数据展示
      loadScripts(currentCategory);
    }

    editorStatus.textContent = '已保存';
    updateSaveButton();
  } catch (e) {
    alert('保存请求失败');
  } finally {
    saving = false;
    btnSave.textContent = '保存';
    updateSaveButton();
  }
});

// ===== 删除脚本按钮 =====
btnDeleteScript.addEventListener('click', () => {
  if (!currentScriptName) return;
  deleteScript(currentScriptName);
});

// ===== 返回列表 =====
btnBackList.addEventListener('click', () => {
  if (isDirty()) {
    if (!confirm('有未保存的更改，确定返回？')) return;
  }
  currentScriptName = '';
  currentScript = null;
  pristineScript = null;
  editorViewEl.style.display = 'none';
  scriptListView.style.display = '';
  if (editorView) {
    editorView.destroy();
    editorView = null;
  }
});

// ===== 工具函数 =====
function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

// ===== 初始化 =====
initEditor();
loadCategories();
