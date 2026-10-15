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
    const result = await Promise.race([response, new Promise((_, reject) => setTimeout(() => reject(new Error(action + ' timed out')), 6000))]);
    assert(result.ok, result.error);
  }
  const result = {};
  const report = value => fetch('/api/desktop/native-verification', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + window.__MYWEBSCRCPY_DESKTOP_TOKEN__ },
    body: JSON.stringify({ player_windows: value }),
  });
  try {
    assert((await invoke('list_player_windows')).recoveryShortcut, 'recovery shortcut missing');
    let rejected = false;
    try { await invoke('set_player_opacity', { opacity: 0, expectedRevision: 0 }); } catch { rejected = true; }
    assert(rejected, 'main window opacity changed');
    result.mainProtected = true;
    for (const serial of ['verify-a', 'verify-b']) {
      await invoke('open_device_window', { serial, width: 380, height: 700 });
    }
    for (let count = 0; ready.size < 2 && count < 100; count++) await new Promise(resolve => setTimeout(resolve, 100));
    assert(ready.size === 2, 'player frontend not mounted');
    for (const serial of ['verify-a', 'verify-b']) await request(serial, 'enable');
    let snapshot = await invoke('list_player_windows');
    assert(snapshot.windows.length === 2 && snapshot.windows.every(player => player.opacity === 0 && player.clickThrough), 'independent window state mismatch');
    result.zeroAndHalfOpacity = true;
    result.transparentBody = true;
    result.clickThroughIPC = true;
    result.rangeGuard = true;
    if (window.__verifyHotkey) {
      const generation = snapshot.recoveryGeneration;
      await report({ stage: 'awaiting_hotkey', shortcut: snapshot.recoveryShortcut });
      for (let count = 0; count < 200; count++) {
        snapshot = await invoke('list_player_windows');
        if (snapshot.recoveryGeneration > generation && snapshot.windows.length === 2 && snapshot.windows.every(player => player.opacity === 100 && !player.clickThrough)) break;
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      assert(snapshot.recoveryGeneration > generation && snapshot.windows.length === 2 && snapshot.windows.every(player => player.opacity === 100 && !player.clickThrough), 'global hotkey did not restore all players');
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
      if (snapshot.windows.length === 2 && snapshot.windows.every(player => player.opacity === 100 && !player.clickThrough)) break;
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert(snapshot.windows.length === 2 && snapshot.windows.every(player => player.opacity === 100 && !player.clickThrough), 'menu recovery failed');
    for (const serial of ['verify-a', 'verify-b']) await request(serial, 'restored');
    result.menuRecovery = true;
    result.ok = true;
  } catch (error) {
    result.error = String(error);
    result.ok = false;
  }
  channel.close();
  await report(result);
})();
