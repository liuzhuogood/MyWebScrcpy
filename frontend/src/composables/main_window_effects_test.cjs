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
  const state = { alwaysOnTop: false, opacity: 100, clickThrough: false };
  const shortcut = 'Option + P';
  const module = { exports: {} };
  const code = ts.transpileModule(fs.readFileSync(path.join(__dirname, 'useMainWindowEffects.ts'), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  let eventHandler;

  vm.runInNewContext(code, {
    module, exports: module.exports,
    console: { warn() {}, error() {} },
    document: {
      documentElement: {
        classList: { add() {}, remove() {} },
        style: { setProperty() {}, removeProperty() {} },
      },
    },
    require(name) {
      if (name === 'vue') return { ...vue, onMounted: callback => mounts.push(callback), onUnmounted: callback => unmounts.push(callback) };
      if (name === 'vue-sonner') return { toast: { error: message => errors.push(message) } };
      if (name === '@tauri-apps/api/event') return {
        async listen(event, handler) {
          eventHandler = handler;
          return () => {};
        },
      };
      if (name === '@tauri-apps/api/core') return {
        isTauri: () => native,
        async invoke(command, args) {
          calls.push({ command, ...args });
          if (command === 'get_main_window_state') return JSON.parse(JSON.stringify(state));
          if (command === 'list_player_windows') return { recoveryShortcut: shortcut };
          if (command === 'set_main_window_opacity') state.opacity = args.opacity;
          if (command === 'set_main_window_click_through') state.clickThrough = args.enabled;
          if (command === 'set_main_window_always_on_top') state.alwaysOnTop = args.alwaysOnTop;
          if (command === 'restore_main_window') {
            state.alwaysOnTop = false;
            state.opacity = 100;
            state.clickThrough = false;
          }
        },
      };
      throw new Error(`Unexpected import ${name}`);
    },
  });
  return {
    use: module.exports.useMainWindowEffects, mounts, unmounts, calls, state, errors,
    triggerEvent(payload) { if (eventHandler) eventHandler({ payload }); },
  };
}

(async () => {
  // 1. Web 模式
  const web = fixture(false);
  const webEffects = web.use();
  web.mounts.forEach(c => c());
  assert.equal(webEffects.native, false);
  assert.equal(web.calls.length, 0, 'web 模式不调用桌面 IPC');

  // 2. 桌面模式
  const desktop = fixture(true);
  const effects = desktop.use();
  assert.equal(effects.native, true);
  for (const m of desktop.mounts) await m();

  assert.equal(effects.shortcut.value, 'Option + P');
  assert.equal(effects.state.value.opacity, 100);

  // 3. 调节透明度
  await effects.setOpacity(80);
  assert.equal(effects.state.value.opacity, 80);
  assert.ok(desktop.calls.some(c => c.command === 'set_main_window_opacity' && c.opacity === 80));

  // 4. 切换置顶
  await effects.toggleAlwaysOnTop();
  assert.equal(effects.state.value.alwaysOnTop, true);
  assert.ok(desktop.calls.some(c => c.command === 'set_main_window_always_on_top' && c.alwaysOnTop === true));

  // 5. 开启穿透
  await effects.setClickThrough(true);
  assert.equal(effects.state.value.clickThrough, true);
  assert.ok(desktop.calls.some(c => c.command === 'set_main_window_click_through' && c.enabled === true));

  // 6. 恢复
  await effects.restore();
  assert.equal(effects.state.value.opacity, 100);
  assert.equal(effects.state.value.alwaysOnTop, false);
  assert.equal(effects.state.value.clickThrough, false);
  assert.ok(desktop.calls.some(c => c.command === 'restore_main_window'));

  // 7. 外部事件同步
  desktop.triggerEvent({ alwaysOnTop: true, opacity: 60, clickThrough: false });
  assert.equal(effects.state.value.alwaysOnTop, true);
  assert.equal(effects.state.value.opacity, 60);

  console.log('PASS: main window effects desktop IPC, state sync, and recovery');
})();
