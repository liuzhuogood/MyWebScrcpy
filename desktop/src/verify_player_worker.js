if (!window.__playerVerifyStarted && document.querySelector('canvas') && document.documentElement.classList.contains('desktop-player')) {
  window.__playerVerifyStarted = true;
  const channel = new BroadcastChannel('player-window-verification');
  const instance = crypto.randomUUID();
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
  window.addEventListener('pagehide', () => channel.close(), { once: true });
  channel.onmessage = async ({ data }) => {
    if (data.serial !== serial || data.instance !== instance) return;
    try {
      if (data.action === 'frame_controls') {
        const reveal = () => document.querySelector('[aria-label="展开工具条"]')?.parentElement.dispatchEvent(new MouseEvent('mouseenter'));
        reveal();
        await waitFor(() => document.querySelector('[aria-label="窗口设置"]'), 'window toolbar did not open');
        const canvas = document.querySelector('canvas');
        canvas.width = 1080; canvas.height = 2400;
        await invoke('fit_player_window', {videoWidth:1080, videoHeight:2400});
        window.dispatchEvent(new Event('resize'));
        await frames();
        const viewport = document.querySelector('[data-player-viewport]').getBoundingClientRect();
        const before = canvas.getBoundingClientRect();
        assert(Math.abs(viewport.width/viewport.height - 1080/2400) < .003, 'native viewport does not fit video aspect');
        assert(Math.abs(before.width - viewport.width) < 2 && Math.abs(before.height - viewport.height) < 2, 'canvas leaves black edges');
        document.querySelector('header').dispatchEvent(new MouseEvent('mouseleave'));
        await waitFor(() => !document.querySelector('header'), 'toolbar did not auto-hide');
        const hidden = canvas.getBoundingClientRect();
        reveal();
        await waitFor(() => document.querySelector('[aria-label="窗口设置"]'), 'toolbar did not reveal');
        const after = canvas.getBoundingClientRect();
        assert(before.width === hidden.width && before.height === hidden.height && hidden.width === after.width && hidden.height === after.height, 'toolbar resized canvas');
        assert(getComputedStyle(document.querySelector('header')).position === 'absolute', 'toolbar occupies layout');
        document.querySelector('[aria-label="窗口设置"]').click();
        await waitFor(() => document.querySelector('[aria-label="小窗透明度"]'), 'window settings did not open');
        assert(!document.querySelector('[aria-label="无边框"]'), 'frame toggle remains');
        document.querySelector('[aria-label="窗口设置"]').click();
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
        assert(player.opacity === 100 && !player.clickThrough && player.borderless && player.toolbarVisible, 'restore state mismatch');
        assert(!document.querySelector('[aria-label="无边框"]'), 'frame toggle returned after recovery');
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
  channel.postMessage({ serial, instance, action: 'ready', ok: true });
}
