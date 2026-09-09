(function () {
  'use strict';
  const canvas = document.getElementById('screen');
  const overlay = document.getElementById('ui-inspector-overlay');
  const serial = new URLSearchParams(location.search).get('serial');
  const button = document.getElementById('btn-inspector');
  if (!canvas || !overlay || !button) return;

  let panel, tabColor, tabXML, readout, tree, props, xpath, xpathResult, message;
  let xmlDoc = null, selected = null, boxes = [], lastPoint = null, raf = 0, magnifier;
  const nodeRows = new WeakMap();

  function el(tag, text, cls) {
    const node = document.createElement(tag);
    if (text != null) node.textContent = text;
    if (cls) node.className = cls;
    return node;
  }
  function build() {
    panel = el('section', null, 'ui-inspector-panel'); panel.hidden = true; panel.setAttribute('role', 'dialog');
    const head = el('div', null, 'ui-inspector-head'); head.append(el('strong', '检查')); const close = el('button', '关闭'); close.onclick = closePanel; head.append(close); panel.append(head);
    const tabs = el('div', null, 'ui-inspector-tabs'); const colorBtn = el('button', '取色坐标'); const xmlBtn = el('button', 'XML 树'); colorBtn.classList.add('active'); tabs.append(colorBtn, xmlBtn); panel.append(tabs);
    tabColor = el('div', null, 'ui-inspector-tab active'); readout = el('div', null, 'ui-inspector-readout'); readout.append(el('span', '坐标'), el('span', '移动鼠标到画面上')); readout.append(el('span', '颜色'), el('span', '—')); tabColor.append(readout); magnifier = document.createElement('canvas'); magnifier.width = magnifier.height = 156; magnifier.className = 'ui-inspector-magnifier'; tabColor.append(magnifier); panel.append(tabColor);
    tabXML = el('div', null, 'ui-inspector-tab'); const refresh = el('button', '刷新 XML'); refresh.onclick = fetchXML; xpath = document.createElement('input'); xpath.className = 'ui-inspector-xpath'; xpath.placeholder = "例如 //*[@resource-id='app:id/login']"; xpath.onkeydown = event => { if (event.key === 'Enter') testXPath(); }; const test = el('button', '测试 XPath'); test.onclick = testXPath; const clear = el('button', '清除'); clear.onclick = clearXPath; xpathResult = el('div', '尚未测试', 'ui-inspector-message'); message = el('div', '', 'ui-inspector-message'); tree = el('ul', null, 'ui-inspector-tree'); props = el('div', null, 'ui-inspector-props'); tabXML.append(refresh, xpath, test, clear, xpathResult, message, tree, props); panel.append(tabXML); document.body.append(panel);
    colorBtn.onclick = () => switchTab(colorBtn, xmlBtn, tabColor, tabXML); xmlBtn.onclick = () => { switchTab(xmlBtn, colorBtn, tabXML, tabColor); if (!xmlDoc) fetchXML(); };
    button.onclick = () => { panel.hidden = !panel.hidden; button.setAttribute('aria-expanded', String(!panel.hidden)); if (!panel.hidden) { setMoreMenuClosed(); syncOverlay(); } else clearOverlay(); };
  }
  function switchTab(active, other, pane, otherPane) { active.classList.add('active'); other.classList.remove('active'); pane.classList.add('active'); otherPane.classList.remove('active'); }
  function setMoreMenuClosed() { const menu = document.getElementById('more-menu'); if (menu) menu.hidden = true; }
  function clearOverlay() { overlay.getContext('2d').clearRect(0, 0, overlay.width, overlay.height); }
  function closePanel() { panel.hidden = true; button.setAttribute('aria-expanded', 'false'); if (selected) selected.row.classList.remove('selected'); selected = null; boxes = []; clearOverlay(); }
  function syncOverlay() { const canvasRect = canvas.getBoundingClientRect(); const parent = overlay.offsetParent || canvas.parentElement; const parentRect = parent.getBoundingClientRect(); overlay.width = canvas.width; overlay.height = canvas.height; overlay.style.left = `${canvasRect.left - parentRect.left}px`; overlay.style.top = `${canvasRect.top - parentRect.top}px`; overlay.style.width = `${canvasRect.width}px`; overlay.style.height = `${canvasRect.height}px`; }
  function sourcePoint(event) { const rect = canvas.getBoundingClientRect(); return { x: Math.max(0, Math.min(canvas.width - 1, Math.floor((event.clientX - rect.left) * canvas.width / rect.width))), y: Math.max(0, Math.min(canvas.height - 1, Math.floor((event.clientY - rect.top) * canvas.height / rect.height))) }; }
  function updateColor(event) { if (!canvas.width || !canvas.height) return; lastPoint = sourcePoint(event); if (!raf) raf = requestAnimationFrame(drawColor); }
  function drawColor() { raf = 0; if (!lastPoint) return; const pixel = canvas.getContext('2d').getImageData(lastPoint.x, lastPoint.y, 1, 1).data; const hex = '#' + [pixel[0], pixel[1], pixel[2]].map(value => value.toString(16).padStart(2, '0')).join('').toUpperCase(); readout.children[1].textContent = `${lastPoint.x} × ${lastPoint.y}`; readout.children[3].textContent = `${hex}  rgb(${pixel[0]}, ${pixel[1]}, ${pixel[2]})`; const context = magnifier.getContext('2d'); context.imageSmoothingEnabled = false; const size = 13, sx = Math.max(0, Math.min(canvas.width - size, lastPoint.x - 6)), sy = Math.max(0, Math.min(canvas.height - size, lastPoint.y - 6)); context.clearRect(0, 0, 156, 156); context.drawImage(canvas, sx, sy, size, size, 0, 0, 156, 156); context.strokeStyle = '#fff'; context.lineWidth = 2; context.strokeRect((lastPoint.x - sx) * 12, (lastPoint.y - sy) * 12, 12, 12); }
  canvas.addEventListener('mousemove', updateColor); canvas.addEventListener('mouseleave', () => { lastPoint = null; });
  // Capture before player.html's mousedown handler so an inspector hit is read-only.
  canvas.addEventListener('mousedown', handleInspectorMouseDown, true);

  function fetchXML() { message.className = 'ui-inspector-message'; message.textContent = '正在获取 XML…'; if (selected) selected.row.classList.remove('selected'); selected = null; boxes = []; xpath.value = ''; xpathResult.className = 'ui-inspector-message'; xpathResult.textContent = '尚未测试'; tree.replaceChildren(); props.textContent = ''; clearOverlay(); fetch(`/api/ui/xml?serial=${encodeURIComponent(serial || '')}`).then(response => response.ok ? response.json() : response.json().then(data => Promise.reject(new Error(data.message || '获取失败')))).then(data => { const parsed = new DOMParser().parseFromString(data.xml, 'application/xml'); if (parsed.querySelector('parsererror')) throw new Error('XML 格式无效'); xmlDoc = parsed; renderTree(); props.textContent = `快照时间：${data.captured_at}`; message.textContent = ''; syncOverlay(); }).catch(error => { xmlDoc = null; tree.replaceChildren(); props.textContent = ''; message.className = 'ui-inspector-message ui-inspector-error'; message.textContent = error.message; }); }
  function renderTree() { tree.replaceChildren(); const root = xmlDoc && xmlDoc.documentElement; if (!root) { message.textContent = '设备未提供可检查节点'; return; } tree.append(renderNode(root)); }
  function renderNode(node) { const item = el('li'); const children = Array.from(node.children); const row = el('div', null, 'ui-inspector-node'); const toggle = el('button', children.length ? '▸' : '', 'ui-inspector-toggle'); toggle.type = 'button'; toggle.setAttribute('aria-expanded', 'false'); const summary = [node.tagName]; ['class', 'resource-id', 'text', 'bounds'].forEach(name => { const value = node.getAttribute(name); if (value) summary.push(`${name}=${value}`); }); row.append(toggle, el('span', summary.join(' '))); row.title = summary.join(' '); nodeRows.set(node, row); row.onclick = () => selectNode(node, row); item.append(row); if (children.length) { const childList = el('ul'); childList.hidden = true; children.forEach(child => childList.append(renderNode(child))); toggle.onclick = event => { event.stopPropagation(); childList.hidden = !childList.hidden; toggle.textContent = childList.hidden ? '▸' : '▾'; toggle.setAttribute('aria-expanded', String(!childList.hidden)); }; item.append(childList); } return item; }
  function bounds(node) { const match = /^\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]$/.exec(node.getAttribute('bounds') || ''); return match ? { x: +match[1], y: +match[2], w: +match[3] - +match[1], h: +match[4] - +match[2], node } : null; }
  function selectNode(node, row) { if (selected && selected.node === node) { selected.row.classList.remove('selected'); selected = null; props.textContent = ''; drawBoxes(); return; } if (selected) selected.row.classList.remove('selected'); selected = { node, row }; row.classList.add('selected'); const attributes = Array.from(node.attributes).map(attribute => `${attribute.name}=${JSON.stringify(attribute.value)}`).join('\n'); props.textContent = `${attributes}\nXPath: ${makeXPath(node)}`; drawBoxes(); }
  function findBoxAtPoint(event) {
    if (panel.hidden || !tabXML.classList.contains('active') || !xmlDoc) return null;
    const point = sourcePoint(event);
    return boxes.concat(selected ? [bounds(selected.node)].filter(Boolean) : [])
      .filter(box => point.x >= box.x && point.x <= box.x + box.w && point.y >= box.y && point.y <= box.y + box.h)
      .sort((a, b) => a.w * a.h - b.w * b.h)[0] || null;
  }
  function handleInspectorMouseDown(event) {
    if (event.button !== 0) return;
    const box = findBoxAtPoint(event);
    if (!box) return;
    const row = nodeRows.get(box.node);
    if (!row) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    selectNode(box.node, row);
  }
  function makeXPath(node) { const resourceID = node.getAttribute('resource-id'); if (resourceID) { const matches = Array.from(xmlDoc.getElementsByTagName('*')).filter(item => item.getAttribute('resource-id') === resourceID); if (matches.length === 1) return `//*[@resource-id='${resourceID.replace(/'/g, "&apos;")}']`; } const parts = []; for (let current = node; current && current.nodeType === 1; current = current.parentElement) { let index = 1; for (let sibling = current.previousElementSibling; sibling; sibling = sibling.previousElementSibling) if (sibling.tagName === current.tagName) index++; parts.unshift(`${current.tagName.toLowerCase()}[${index}]`); } return '/' + parts.join('/'); }
  function drawBoxes() { if (!panel || panel.hidden) return; syncOverlay(); const context = overlay.getContext('2d'); context.clearRect(0, 0, overlay.width, overlay.height); context.strokeStyle = '#00e676'; context.lineWidth = Math.max(2, canvas.width / 500); boxes.forEach(box => context.strokeRect(box.x, box.y, box.w, box.h)); if (selected) { const box = bounds(selected.node); if (box) { context.lineWidth = Math.max(3, canvas.width / 350); context.strokeRect(box.x, box.y, box.w, box.h); } } }
  function clearXPath() { xpath.value = ''; boxes = []; xpathResult.className = 'ui-inspector-message'; xpathResult.textContent = '尚未测试'; drawBoxes(); }
  function testXPath() { if (!xmlDoc || !xpath.value.trim()) { clearXPath(); return; } try { const result = xmlDoc.evaluate(xpath.value, xmlDoc, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null); boxes = []; for (let index = 0; index < result.snapshotLength; index++) { const box = bounds(result.snapshotItem(index)); if (box) boxes.push(box); } xpathResult.className = 'ui-inspector-message'; xpathResult.textContent = `${result.snapshotLength} 个匹配元素`; drawBoxes(); } catch (_) { boxes = []; xpathResult.className = 'ui-inspector-message ui-inspector-error'; xpathResult.textContent = 'XPath 无效'; drawBoxes(); } }
  build(); window.addEventListener('resize', drawBoxes); document.addEventListener('fullscreenchange', () => setTimeout(drawBoxes, 100));
}());
