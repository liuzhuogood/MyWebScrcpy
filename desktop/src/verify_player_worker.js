if (!window.__playerVerifyStarted && document.querySelector('canvas') && document.documentElement.classList.contains('desktop-player')) {
  window.__playerVerifyStarted = true;
  const channel = new BroadcastChannel('player-window-verification');
  const serial = new URL(location.href).searchParams.get('serial');
  const invoke = async (command, args) => {
    if (command === 'set_player_opacity' || command === 'set_player_click_through') {
      const snapshot = await window.__TAURI_INTERNALS__.invoke('list_player_windows');
      args.expectedRevision = snapshot.windows.find(player => player.serial === serial).revision;
    }
    return window.__TAURI_INTERNALS__.invoke(command, args);
  };
  // WebKit suspends animation frames once the root becomes fully invisible.
  const frames = () => new Promise(resolve => setTimeout(resolve, 80));
  const assert = (value, message) => { if (!value) throw new Error(message); };
  channel.onmessage = async ({ data }) => {
    if (data.serial !== serial) return;
    try {
      if (data.action === 'enable') {
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
        assert(player.opacity === 100 && !player.clickThrough, 'restore state mismatch');
        let rejected = false;
        try {
          await window.__TAURI_INTERNALS__.invoke('set_player_opacity', { opacity: 0, expectedRevision: player.revision - 1 });
        } catch { rejected = true; }
        assert(rejected, 'a command queued before recovery was accepted');
      }
      channel.postMessage({ serial, action: data.action, ok: true });
    } catch (error) {
      channel.postMessage({ serial, action: data.action, error: String(error) });
    }
  };
  channel.postMessage({ serial, action: 'ready', ok: true });
}
