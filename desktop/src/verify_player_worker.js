if (!window.__playerVerifyStarted && document.querySelector('canvas') && document.documentElement.classList.contains('desktop-player')) {
  window.__playerVerifyStarted = true;
  const channel = new BroadcastChannel('player-window-verification');
  const serial = new URL(location.href).searchParams.get('serial');
  const invoke = async (command, args) => {
    if (['set_player_opacity', 'set_player_click_through', 'set_player_borderless', 'set_player_toolbar_visible'].includes(command)) {
      const snapshot = await window.__TAURI_INTERNALS__.invoke('list_player_windows');
      args.expectedRevision = snapshot.windows.find(player => player.serial === serial).revision;
    }
    return window.__TAURI_INTERNALS__.invoke(command, args);
  };
  // WebKit suspends animation frames once the root becomes fully invisible.
  const frames = () => new Promise(resolve => setTimeout(resolve, 80));
  const assert = (value, message) => { if (!value) throw new Error(message); };
  const playerState = async () => (await invoke('list_player_windows')).windows.find(player => player.serial === serial);
  async function waitFor(check, message) {
    for (let count = 0; count < 40; count++) {
      if (await check()) return;
      await frames();
    }
    throw new Error(message);
  }
  channel.onmessage = async ({ data }) => {
    if (data.serial !== serial) return;
    try {
      if (data.action === 'frame_controls') {
        document.querySelector('[aria-label="窗口设置"]').click();
        await waitFor(() => document.querySelector('[role="switch"][aria-label="无边框"]'), 'window settings did not open');
        let toggle = document.querySelector('[role="switch"][aria-label="无边框"]');
        assert(toggle.getAttribute('aria-checked') === 'true', 'initial borderless default missing');
        toggle.click();
        await waitFor(async () => !(await playerState()).borderless, 'popover switch did not restore frame');
        await frames();
        toggle.click();
        await waitFor(async () => (await playerState()).borderless, 'popover switch did not remove frame');
        document.querySelector('[aria-label="窗口设置"]').click();
        await frames();
        document.querySelector('[aria-label="更多"]').click();
        await waitFor(() => [...document.querySelectorAll('[role="menuitem"]')].some(item => item.textContent.trim() === '设置'), 'settings menu item missing');
        [...document.querySelectorAll('[role="menuitem"]')].find(item => item.textContent.trim() === '设置').click();
        await waitFor(() => document.querySelector('aside[aria-label="投屏设置"]'), 'player settings did not open');
        toggle = document.querySelector('aside[aria-label="投屏设置"] [aria-label="无边框"]');
        assert(toggle.getAttribute('aria-checked') === 'true', 'settings sidebar did not share window state');
        toggle.click();
        await waitFor(async () => !(await playerState()).borderless, 'sidebar switch did not restore frame');
        await frames();
        toggle.click();
        await waitFor(async () => (await playerState()).borderless, 'sidebar switch did not remove frame');
        document.querySelector('[aria-label="关闭设置"]').click();
        document.querySelector('[aria-label="隐藏工具条"]').click();
        await waitFor(() => !document.querySelector('[aria-label="隐藏工具条"]'), 'toolbar did not hide');
        await waitFor(() => document.querySelector('[data-sonner-toast]'), 'hide toolbar toast missing');
        const text = document.querySelector('[data-sonner-toast]').textContent;
        const snapshot = await invoke('list_player_windows');
        assert(text.includes(snapshot.recoveryShortcut) && text.includes('视图 → 恢复所有小窗'), 'toast recovery instructions incorrect');
      }
      if (data.action === 'enable') {
        await invoke('set_player_borderless', { enabled: true });
        await invoke('set_player_toolbar_visible', { visible: false });
        await invoke('set_player_opacity', { opacity: 0 });
        await frames();
        assert(getComputedStyle(document.querySelector('#app')).opacity === '0', '0% did not render');
        assert(getComputedStyle(document.body).backgroundColor === 'rgba(0, 0, 0, 0)', 'body is opaque');
        await invoke('set_player_opacity', { opacity: 50 });
        await frames();
        assert(getComputedStyle(document.querySelector('#app')).opacity === '0.5', '50% did not render');
        let rejected = false;
        try { await invoke('set_player_opacity', { opacity: 101 }); } catch { rejected = true; }
        assert(rejected, '101% was accepted');
        await invoke('set_player_opacity', { opacity: 0 });
        await invoke('set_player_click_through', { enabled: true });
      }
      if (data.action === 'restored') {
        await frames();
        assert(getComputedStyle(document.querySelector('#app')).opacity === '1', 'restore did not render');
        const snapshot = await invoke('list_player_windows');
        const player = snapshot.windows.find(item => item.serial === serial);
        assert(player.opacity === 100 && !player.clickThrough && !player.borderless && player.toolbarVisible, 'restore state mismatch');
        await waitFor(() => document.querySelector('[aria-label="隐藏工具条"]'), 'recovery did not render the toolbar');
        for (const [command, args] of [
          ['set_player_opacity', { opacity: 0 }],
          ['set_player_borderless', { enabled: true }],
          ['set_player_toolbar_visible', { visible: false }],
        ]) {
          let rejected = false;
          try {
            await window.__TAURI_INTERNALS__.invoke(command, { ...args, expectedRevision: player.revision - 1 });
          } catch { rejected = true; }
          assert(rejected, 'a command queued before recovery was accepted: ' + command);
        }
      }
      channel.postMessage({ serial, action: data.action, ok: true });
    } catch (error) {
      channel.postMessage({ serial, action: data.action, error: String(error) });
    }
  };
  channel.postMessage({ serial, action: 'ready', ok: true });
}
