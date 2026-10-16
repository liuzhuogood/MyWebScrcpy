(async () => {
  const invoke = (command, args) => window.__TAURI_INTERNALS__.invoke(command, args);
  const assert = (value, message) => { if (!value) throw new Error(message); };
  const channel = new BroadcastChannel('player-window-verification');
  const ready = new Set();
  const waiting = new Map();
  channel.onmessage = ({ data }) => {
    if (data.action === 'ready') ready.add(data.serial);
    const resolve = waiting.get(data.serial + ':' + data.action);
    if (resolve) { waiting.delete(data.serial + ':' + data.action); resolve(data); }
  };
  async function request(serial, action) {
    const response = new Promise(resolve => waiting.set(serial + ':' + action, resolve));
    channel.postMessage({ serial, action });
    const result = await Promise.race([response, new Promise((_, reject) => setTimeout(() => reject(new Error(action + ' timed out')), action === 'frame_controls' ? 15000 : 6000))]);
    assert(result.ok, result.error);
  }
  const result = {};
  const report = value => fetch('/api/desktop/native-verification', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + window.__MYWEBSCRCPY_DESKTOP_TOKEN__ },
    body: JSON.stringify({ player_windows: value }),
  });
  let frameRequest = 0;
  async function checkNativeFrames(expected) {
    const id = ++frameRequest;
    window.__nativeFramesResult = null;
    await report({ stage: 'check_frames', id, expected });
    for (let count = 0; window.__nativeFramesResult?.id !== id && count < 60; count++) await new Promise(resolve => setTimeout(resolve, 100));
    assert(window.__nativeFramesResult?.id === id && window.__nativeFramesResult.valid === true, 'native window decorations do not match borderless state');
  }
  const restored = player => player.opacity === 100 && !player.clickThrough && !player.borderless && player.toolbarVisible;
  try {
    assert((await invoke('list_player_windows')).recoveryShortcut, 'recovery shortcut missing');
    let rejected = false;
    try { await invoke('set_player_opacity', { opacity: 0, expectedRevision: 0 }); } catch { rejected = true; }
    assert(rejected, 'main window opacity changed');
    for (const [command, args] of [['set_player_borderless', { enabled: true }], ['set_player_toolbar_visible', { visible: false }]]) {
      rejected = false;
      try { await invoke(command, { ...args, expectedRevision: 0 }); } catch { rejected = true; }
      assert(rejected, 'main window accepted ' + command);
    }
    result.mainProtected = true;
    for (const serial of ['verify-a', 'verify-b']) {
      await invoke('open_device_window', { serial, width: 380, height: 700, borderless: serial === 'verify-a' });
    }
    for (let count = 0; ready.size < 2 && count < 100; count++) await new Promise(resolve => setTimeout(resolve, 100));
    assert(ready.size === 2, 'player frontend not mounted');
    await checkNativeFrames({ 'player-verify_a': true, 'player-verify_b': false });
    result.borderlessDefault = true;
    await request('verify-a', 'frame_controls');
    result.borderlessControls = true;
    result.toolbarToast = true;
    for (const serial of ['verify-a', 'verify-b']) await request(serial, 'enable');
    let snapshot = await invoke('list_player_windows');
    assert(snapshot.windows.length === 2 && snapshot.windows.every(player => player.opacity === 0 && player.clickThrough && player.borderless && !player.toolbarVisible), 'independent window state mismatch');
    await checkNativeFrames({ 'player-verify_a': true, 'player-verify_b': true });
    result.zeroAndHalfOpacity = true;
    result.transparentBody = true;
    result.clickThroughIPC = true;
    result.rangeGuard = true;
    if (window.__verifyHotkey) {
      const generation = snapshot.recoveryGeneration;
      await report({ stage: 'awaiting_hotkey', shortcut: snapshot.recoveryShortcut });
      for (let count = 0; count < 200; count++) {
        snapshot = await invoke('list_player_windows');
        if (snapshot.recoveryGeneration > generation && snapshot.windows.length === 2 && snapshot.windows.every(restored)) break;
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      assert(snapshot.recoveryGeneration > generation && snapshot.windows.length === 2 && snapshot.windows.every(restored), 'global hotkey did not restore all players');
      for (const serial of ['verify-a', 'verify-b']) {
        await request(serial, 'restored');
        await request(serial, 'enable');
      }
      result.globalHotkey = true;
    }
    await invoke('restore_player_window', { serial: 'verify-a' });
    await request('verify-a', 'restored');
    snapshot = await invoke('list_player_windows');
    assert(snapshot.windows.find(player => player.serial === 'verify-b').clickThrough, 'restoring A changed B');
    result.independentRecovery = true;
    // Reopening an existing invisible player restores it before focusing it.
    await invoke('open_device_window', { serial: 'verify-b' });
    await request('verify-b', 'restored');
    result.reopenRecovery = true;
    ready.delete('verify-a');
    await report({ stage: 'close_a' });
    for (let count = 0; count < 60; count++) {
      snapshot = await invoke('list_player_windows');
      if (snapshot.windows.length === 1) break;
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert(snapshot.windows.length === 1, 'closed window state was not removed');
    await invoke('open_device_window', { serial: 'verify-a' });
    for (let count = 0; !ready.has('verify-a') && count < 60; count++) await new Promise(resolve => setTimeout(resolve, 100));
    assert(ready.has('verify-a'), 'recreated window did not mount');
    await request('verify-a', 'restored');
    result.closeReopen = true;
    for (const serial of ['verify-a', 'verify-b']) await request(serial, 'enable');
    await report({ stage: 'restore_menu' });
    for (let count = 0; count < 60; count++) {
      snapshot = await invoke('list_player_windows');
      if (snapshot.windows.length === 2 && snapshot.windows.every(restored)) break;
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert(snapshot.windows.length === 2 && snapshot.windows.every(restored), 'menu recovery failed');
    for (const serial of ['verify-a', 'verify-b']) await request(serial, 'restored');
    result.menuRecovery = true;
    await checkNativeFrames({ 'player-verify_a': false, 'player-verify_b': false });
    result.toolbarRecovery = true;
    result.nativeDecorations = true;
    result.ok = true;
  } catch (error) {
    result.error = String(error);
    result.ok = false;
  }
  channel.close();
  await report(result);
})();
