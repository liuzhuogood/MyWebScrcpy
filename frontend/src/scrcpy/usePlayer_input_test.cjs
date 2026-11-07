const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const ts = require('typescript');
function target() {
  const listeners = new Map();
  return {
    addEventListener(type, cb) { if (!listeners.has(type)) listeners.set(type, new Set()); listeners.get(type).add(cb); },
    removeEventListener(type, cb) { listeners.get(type)?.delete(cb); },
    fire(type, props = {}) { const e = { preventDefault() {}, ...props }; for (const cb of listeners.get(type) ?? []) cb(e); },
    listenerCount(type) { return listeners.get(type)?.size ?? 0; },
  };
}
async function fixture(mode = 'sdk') {
  const mounted = [], unmounted = [], frames = new Map(), timers = new Map(), sockets = [], broadcasts = [], logs = [];
  let next = 0, channel;
  const window = Object.assign(target(), {
    setTimeout(cb) { const id = ++next; timers.set(id, cb); return id; }, clearTimeout(id) { timers.delete(id); },
    setInterval() { return ++next; }, clearInterval() {},
  });
  const document = Object.assign(target(), { activeElement: null });
  const canvas = Object.assign(target(), { width: 360, height: 800, style: {}, closest: () => null, getBoundingClientRect: () => ({ left: 0, top: 0, width: 360, height: 800 }) });
  const settings = { injectMode: { value: mode }, bitrate: { value: 4000000 }, maxFps: { value: 30 }, autoAudio: { value: false } };
  class Socket {
    static OPEN = 1; static CONNECTING = 0;
    constructor(url) { this.url = url; this.readyState = 0; this.bufferedAmount = 0; this.sent = []; sockets.push(this); }
    send(bytes) { this.sent.push(typeof bytes === 'string' ? bytes : bytes.slice()); }
    close() { this.readyState = 3; this.onclose?.(); }
    ready() { this.readyState = 1; this.onopen?.(); this.onmessage?.({ data: JSON.stringify({ type: 'meta', width: 1080, height: 2400, codec: 'h264', session_id: 'one' }) }); }
  }
  const helpers = new Map();
  function load(file) {
    if (helpers.has(file)) return helpers.get(file);
    const module = { exports: {} };
    vm.runInNewContext(ts.transpileModule(fs.readFileSync(path.join(__dirname, file + '.ts'), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, { module, exports: module.exports, Uint8Array, DataView, require: n => load(n.replace('./', '')) });
    helpers.set(file, module.exports); return module.exports;
  }
  const module = { exports: {} };
  vm.runInNewContext(ts.transpileModule(fs.readFileSync(path.join(__dirname, 'usePlayer.ts'), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, {
    module, exports: module.exports, Uint8Array, ArrayBuffer, DataView, URLSearchParams, AbortController,
    console: { log() {}, warn(...args) { logs.push(args); }, error(...args) { logs.push(args); } },
    getComputedStyle: () => ({ paddingLeft: '0', paddingRight: '0', paddingTop: '0', paddingBottom: '0' }),
    window, document, WebSocket: Socket, location: { protocol: 'http:', host: 'localhost' },
    localStorage: { getItem() { return null; }, setItem() {} }, performance: { now: () => 0 },
    requestAnimationFrame(cb) { const id = ++next; frames.set(id, cb); return id; }, cancelAnimationFrame(id) { frames.delete(id); },
    ResizeObserver: class { observe() {} disconnect() {} }, Element: class {},
    fetch: async () => ({ ok: true, json: async () => ({ screenOn: true }) }),
    setTimeout: window.setTimeout, clearTimeout: window.clearTimeout, setInterval: window.setInterval, clearInterval: window.clearInterval,
    require(name) {
      if (name === 'vue') return { ref: value => ({ value }), watch() {}, onMounted: cb => mounted.push(cb), onUnmounted: cb => unmounted.push(cb) };
      if (name === '@/inspector/guard') return { inspectorConsumesInput: () => false };
      if (name === './protocol') return load('protocol');
      if (name === './controlSender') return load('controlSender');
      if (name === './decoder') return { ScrcpyDecoder: class { configure() {} close() {} } };
      if (name === './audio') return { DeviceAudio: class { close() {} } };
      if (name === './pip') return { PictureInPicture: class { close() { return Promise.resolve(); } } };
      if (name === './useClipboardSync') return { useClipboardSync: () => ({}) };
      if (name === './useFrameUploader') return { useFrameUploader: () => ({ start() {}, stop() {} }) };
      if (name === './useScreenPower') return { useScreenPower: () => ({ prepareOnOpen: () => Promise.resolve(), observeScreenState() {} }) };
      if (name === '@/composables/usePlayerSettings') return { usePlayerSettings: () => settings };
      if (name === '@/composables/useGroupControl') return { useGroupControl: () => ({ registerChannel(value) { channel = value; return () => {}; }, broadcastControl(...args) { broadcasts.push(args); } }) };
      throw new Error(name);
    },
  });
  const player = module.exports.usePlayer('xiaomi');
  player.canvasRef.value = canvas;
  player.stageRef.value = { clientWidth: 360, clientHeight: 800 };
  mounted.forEach(cb => cb()); await Promise.resolve(); await Promise.resolve();
  sockets[0].ready();
  const frame = () => { const copy = [...frames]; frames.clear(); copy.forEach(([, cb]) => cb()); };
  frame(); frame();
  return { player, canvas, window, settings, sockets, frames, timers, broadcasts, logs, frame, channel, unmount() { unmounted.forEach(cb => cb()); } };
}
(async () => {
  {
    const f = await fixture(); const ws = f.sockets[0];
    f.canvas.fire('mousedown', { button: 0, clientX: 10, clientY: 10 });
    for (let i = 1; i <= 100; i++) f.window.fire('mousemove', { clientX: i + 10, clientY: i + 10 });
    assert.equal(ws.sent.length, 1); assert.equal(f.broadcasts.length, 1, 'raw moves are not broadcast before coalescing');
    f.frame(); assert.equal(ws.sent.length, 2); assert.equal(f.broadcasts.length, 2);
    f.window.fire('mousemove', { clientX: 200, clientY: 200 });
    f.window.fire('mouseup', { clientX: 250, clientY: 250 });
    assert.deepEqual(ws.sent.map(p => p[1]), [0, 2, 2, 1]);
    assert.equal(new DataView(ws.sent.at(-2).buffer).getInt32(10), 750, 'release coordinate is flushed before UP');
    f.frame(); assert.equal(ws.sent.length, 4);
    f.channel.send(new Uint8Array(ws.sent[1])); assert.equal(ws.sent.length, 5); assert.equal(f.broadcasts.length, 4, 'group target sends once with no echo');
    f.canvas.fire('mousedown', { button: 0, clientX: 10, clientY: 10 });
    f.window.fire('mousemove', { clientX: 80, clientY: 80 });
    f.window.fire('blur'); assert.equal(ws.sent.at(-1)[1], 1, 'focus loss releases local pointer');
    const count = ws.sent.length; f.frame(); f.window.fire('mouseup', { clientX: 80, clientY: 80 }); assert.equal(ws.sent.length, count);
    f.unmount(); assert.equal(f.window.listenerCount('blur'), 0); assert.equal(f.canvas.listenerCount('touchcancel'), 0);
  }
  {
    const f = await fixture(); const ws = f.sockets[0];
    const touch = (clientX, clientY) => ({ clientX, clientY, force: 1 });
    f.canvas.fire('touchstart', { touches: [touch(10, 10)] });
    f.canvas.fire('touchmove', { touches: [touch(20, 20)] });
    f.canvas.fire('touchcancel');
    assert.deepEqual(ws.sent.map(p => p[1]), [0, 1], 'touch cancellation releases without replaying pending motion');
    f.frame(); assert.equal(ws.sent.length, 2); f.unmount();
  }
  for (const mode of ['adb', 'hid']) {
    const f = await fixture(mode); const ws = f.sockets[0];
    f.canvas.fire('mousedown', { button: 0, clientX: 10, clientY: 10 });
    f.window.fire('mousemove', { clientX: 100, clientY: 100 }); f.frame();
    assert.equal(ws.sent.length, 0, `${mode} preserves whole-gesture submission`);
    f.window.fire('mouseup', { clientX: 150, clientY: 150 });
    assert.equal(ws.sent.length, 1); assert.equal(f.broadcasts.length, 1);
    const gesture = JSON.parse(ws.sent[0]); assert.equal(gesture.type, mode === 'adb' ? 'input.pointer' : 'input.hid'); assert.equal(gesture.action, 'swipe');
    f.unmount();
  }
  {
    const f = await fixture(); const ws = f.sockets[0];
    f.canvas.fire('mousedown', { button: 0, clientX: 10, clientY: 10 });
    f.window.fire('mousemove', { clientX: 80, clientY: 80 });
    ws.bufferedAmount = 5000; f.window.fire('mouseup', { clientX: 100, clientY: 100 });
    assert.equal(ws.readyState, 3, 'critical command behind congestion causes explicit reconnect');
    assert.ok(f.logs.some(args => args[0] === '[input]'));
    const count = ws.sent.length; f.frame(); assert.equal(ws.sent.length, count, 'old move never leaks to a reconnect'); f.unmount();
  }
  console.log('PASS: usePlayer mouse/touch event bursts, final coordinates, group no-echo, cancellation, unchanged ADB/HID and congestion recovery');
})().catch(error => { console.error(error); process.exitCode = 1; });
