const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const { createRequire } = require('node:module');
const localRequire = createRequire(path.resolve(__dirname, '../../package.json'));
const ts = localRequire('typescript');
const vue = localRequire('vue');

function fixture(native = true) {
  const mounts = [], unmounts = [], calls = [], errors = [];
  const state = { windows: [{ serial: 'a', opacity: 100, clickThrough: false, borderless: false, toolbarVisible: true, revision: 0 }], recoveryShortcut: 'Alt + P' };
  let intervalCount = 0, clearCount = 0, release, holdNext = false;
  const module = { exports: {} };
  const code = ts.transpileModule(fs.readFileSync(path.join(__dirname, 'usePlayerWindows.ts'), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  vm.runInNewContext(code, {
    module, exports: module.exports,
    console: { warn() {}, error() {} },
    setInterval() { intervalCount++; return 1; },
    clearInterval() { clearCount++; },
    require(name) {
      if (name === 'vue') return { ...vue, onMounted: callback => mounts.push(callback), onUnmounted: callback => unmounts.push(callback) };
      if (name === 'vue-sonner') return { toast: { error: message => errors.push(message) } };
      if (name === '@tauri-apps/api/core') return {
        isTauri: () => native,
        async invoke(command, args) {
          calls.push({ command, ...args });
          if (holdNext && command === 'set_player_opacity') {
            holdNext = false;
            await new Promise(resolve => { release = resolve; });
          }
          if (['set_player_opacity', 'set_player_click_through', 'set_player_borderless', 'set_player_toolbar_visible'].includes(command) && args.expectedRevision !== state.windows[0].revision)
            throw new Error('小窗已恢复，请重新调整');
          if (command === 'set_player_opacity') state.windows[0].opacity = args.opacity;
          if (command === 'set_player_click_through') state.windows[0].clickThrough = args.enabled;
          if (command === 'set_player_borderless') state.windows[0].borderless = args.enabled;
          if (command === 'set_player_toolbar_visible') state.windows[0].toolbarVisible = args.visible;
          if (command === 'restore_player_window') Object.assign(state.windows[0], { opacity: 100, clickThrough: false, borderless: false, toolbarVisible: true, revision: state.windows[0].revision + 1 });
          if (command === 'list_player_windows') return JSON.parse(JSON.stringify(state));
        },
      };
      throw new Error(`Unexpected import ${name}`);
    },
  });
  return {
    use: module.exports.usePlayerWindows, mounts, unmounts, calls, state, errors,
    intervals: () => intervalCount, cleared: () => clearCount,
    hold() { holdNext = true; }, release() { release(); },
  };
}

const settle = () => new Promise(resolve => setImmediate(resolve));
(async () => {
  const web = fixture(false);
  web.use(vue.ref('a'));
  web.mounts.forEach(callback => callback());
  web.unmounts.forEach(callback => callback());
  assert.equal(web.calls.length, 0, 'web mode must not invoke desktop IPC');
  assert.equal(web.intervals(), 0);

  const app = fixture();
  const first = app.use(vue.ref('a'));
  const second = app.use(vue.ref('a'));
  app.mounts.forEach(callback => callback());
  await settle();
  assert.equal(app.intervals(), 1, 'device cards share one poller');
  assert.equal(first.needsRecovery.value, false);

  await first.setBorderless(true);
  assert.equal(second.player.value.borderless, true);
  assert.equal(first.needsRecovery.value, true, 'borderless window must offer recovery');
  await first.restore();
  await first.setToolbarVisible(false);
  assert.equal(second.player.value.toolbarVisible, false);
  assert.equal(first.needsRecovery.value, true, 'hidden toolbar must offer recovery');
  await first.restore();
  assert.equal(second.player.value.toolbarVisible, true, 'restore must show the toolbar');
  assert.equal(second.player.value.borderless, false);
  assert.equal(first.needsRecovery.value, false);

  app.hold();
  const dragging = first.setOpacity(50);
  void first.setOpacity(80);
  void first.setOpacity(0);
  app.release();
  await dragging;
  assert.deepEqual(app.calls.filter(call => call.command === 'set_player_opacity').map(call => call.opacity), [50, 0]);
  assert.equal(second.player.value.opacity, 0);
  assert.equal(second.needsRecovery.value, true);

  app.hold();
  const changing = first.setOpacity(20);
  void first.setOpacity(10);
  void first.setClickThrough(true);
  void first.setBorderless(true);
  void first.setToolbarVisible(false);
  void first.restore();
  app.release();
  await changing;
  assert.equal(app.state.windows[0].opacity, 100, 'recovery during a drag must not be dropped');
  assert.equal(app.state.windows[0].clickThrough, false, 'recovery cancels pending click-through');
  assert.equal(app.state.windows[0].borderless, false, 'recovery cancels pending borderless');
  assert.equal(app.state.windows[0].toolbarVisible, true, 'recovery cancels pending toolbar hide');
  assert.equal(first.needsRecovery.value, false);
  assert.equal(app.errors.length, 0);

  app.hold();
  const beforeExternalRecovery = first.setOpacity(50);
  void first.setOpacity(0);
  app.state.windows[0].revision++;
  app.release();
  await beforeExternalRecovery;
  assert.equal(app.state.windows[0].opacity, 100, 'stale drag must not undo external recovery');
  assert.equal(app.errors.length, 1);

  app.unmounts[0]();
  assert.equal(app.cleared(), 0, 'remaining card still needs polling');
  app.unmounts[1]();
  assert.equal(app.cleared(), 1);
  console.log('PASS: desktop-only IPC, shared polling, borderless/toolbar recovery, latest drag value, queued recovery and lifecycle cleanup');
})().catch(error => { console.error(error); process.exitCode = 1; });
