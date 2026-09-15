(() => {
  const main = document.querySelector('.player-main');
  if (!main) return;

  let panel, splitter, active;

  function ensure() {
    if (panel) return;
    splitter = document.createElement('div');
    splitter.className = 'right-panel-resizer';
    splitter.setAttribute('aria-hidden', 'true');

    panel = document.createElement('aside');
    panel.className = 'right-panel';
    panel.hidden = true;
    main.append(splitter, panel);

    splitter.addEventListener('pointerdown', (e) => {
      if (innerWidth <= 820) return;
      e.preventDefault();
      const startX = e.clientX;
      const startWidth = panel.getBoundingClientRect().width;
      main.classList.add('right-panel-resized');
      main.style.setProperty('--right-width', `${startWidth}px`);
      splitter.setPointerCapture(e.pointerId);
      splitter.classList.add('dragging');

      const move = (event) => {
        event.preventDefault();
        const max = Math.max(360, main.clientWidth - 300);
        const width = Math.max(360, Math.min(max, startWidth + startX - event.clientX));
        main.style.setProperty('--right-width', `${width}px`);
        window.dispatchEvent(new Event('resize'));
      };

      const stop = () => {
        splitter.classList.remove('dragging');
        splitter.removeEventListener('pointermove', move);
        splitter.removeEventListener('pointerup', stop);
        splitter.removeEventListener('pointercancel', stop);
        try { splitter.releasePointerCapture(e.pointerId); } catch (_) {}
        window.dispatchEvent(new Event('resize'));
      };

      splitter.addEventListener('pointermove', move);
      splitter.addEventListener('pointerup', stop);
      splitter.addEventListener('pointercancel', stop);
    });
  }

  window.RightPanel = {
    open(node) {
      ensure();

      // 1. 互斥关闭 UI 检查面板
      const inspectorPanel = document.querySelector('.ui-inspector-panel');
      if (inspectorPanel) inspectorPanel.setAttribute('hidden', '');
      main.classList.remove('inspector-open', 'inspector-resized');
      main.style.removeProperty('--inspector-width');
      document.getElementById('btn-inspector')?.setAttribute('aria-expanded', 'false');

      // 2. 互斥关闭 ADB Shell 终端面板
      const shellPanel = document.querySelector('.adb-shell-panel');
      if (shellPanel) shellPanel.setAttribute('hidden', '');
      main.classList.remove('shell-open', 'shell-resized');
      main.style.removeProperty('--shell-width');
      document.getElementById('btn-adb-shell')?.setAttribute('aria-expanded', 'false');

      // 3. 打开前重置宽度，默认自动吃满右侧全部剩余宽度
      main.classList.remove('right-panel-resized');
      main.style.removeProperty('--right-width');

      if (active && active !== node) {
        active.close?.();
        active.hidden = true;
      }
      active = node;
      panel.append(node);
      node.hidden = false;
      if (node instanceof HTMLDialogElement && !node.open) {
        node.show();
      }
      panel.hidden = false;
      main.classList.add('right-panel-open');
      window.dispatchEvent(new Event('resize'));
    },

    close(node) {
      if (node) {
        node.close?.();
        node.hidden = true;
      }
      if (!node || node === active) {
        active = null;
        if (panel) panel.hidden = true;
        main.classList.remove('right-panel-open', 'right-panel-resized');
        main.style.removeProperty('--right-width');
        window.dispatchEvent(new Event('resize'));
      }
    }
  };
})();
