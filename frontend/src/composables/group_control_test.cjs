const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const { createRequire } = require('node:module');
const localRequire = createRequire(path.resolve(__dirname, '../../package.json'));
const ts = localRequire('typescript');
const vue = localRequire('vue');

function loadModule() {
  const module = { exports: {} };
  const rawCode = fs.readFileSync(path.join(__dirname, 'useGroupControl.ts'), 'utf8');
  const code = ts.transpileModule(rawCode, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;

  vm.runInNewContext(code, {
    module,
    exports: module.exports,
    console: { warn() {}, error() {} },
    location: { protocol: 'http:', host: '127.0.0.1:8000' },
    require(name) {
      if (name === 'vue') return vue;
      throw new Error(`Unexpected import: ${name}`);
    },
  });

  return module.exports;
}

(function testGroupControl() {
  const mod = loadModule();
  const { useGroupControl, transformTouchPacket, transformScrollPacket } = mod;

  // 1. 测试 32 字节触摸数据包缩放映射
  // 构造模拟 touch 包：TYPE_INJECT_TOUCH_EVENT = 2, action = 2 (move), x = 540, y = 1200, width = 1080, height = 2400
  const buf = new ArrayBuffer(32);
  const view = new DataView(buf);
  view.setUint8(0, 2);
  view.setUint8(1, 2);
  view.setBigUint64(2, 0n, false);
  view.setInt32(10, 540, false);
  view.setInt32(14, 1200, false);
  view.setUint16(18, 1080, false);
  view.setUint16(20, 2400, false);

  const inputBytes = new Uint8Array(buf);

  // 目标设备尺寸：720 x 1600
  const targetScaled = transformTouchPacket(
    inputBytes,
    { width: 1080, height: 2400 },
    { width: 720, height: 1600 }
  );

  const targetView = new DataView(targetScaled.buffer, targetScaled.byteOffset, targetScaled.byteLength);
  assert.equal(targetView.getInt32(10, false), 360, 'x 坐标应按比例映射到 360 (50%)');
  assert.equal(targetView.getInt32(14, false), 800, 'y 坐标应按比例映射到 800 (50%)');
  assert.equal(targetView.getUint16(18, false), 720, '目标设备宽应更新为 720');
  assert.equal(targetView.getUint16(20, false), 1600, '目标设备高应更新为 1600');

  // 2. 测试组控单例通道广播与分发
  const gc = useGroupControl();
  gc.setActive(true);
  gc.setTargets(['dev-1', 'dev-2', 'dev-3']);

  const dev2Packets = [];
  const dev3Packets = [];

  const unreg2 = gc.registerChannel({
    serial: 'dev-2',
    send: (b) => dev2Packets.push(b),
    getDeviceSize: () => ({ width: 720, height: 1600 }),
  });

  const unreg3 = gc.registerChannel({
    serial: 'dev-3',
    send: (b) => dev3Packets.push(b),
    getDeviceSize: () => ({ width: 1080, height: 2400 }),
  });

  // dev-1 产生触摸操作，向 dev-2 和 dev-3 广播
  gc.broadcastControl('dev-1', inputBytes, { width: 1080, height: 2400 });

  assert.equal(dev2Packets.length, 1, 'dev-2 应收到 1 条广播消息');
  assert.equal(dev3Packets.length, 1, 'dev-3 应收到 1 条广播消息');

  const p2View = new DataView(dev2Packets[0].buffer, dev2Packets[0].byteOffset, dev2Packets[0].byteLength);
  assert.equal(p2View.getInt32(10, false), 360, 'dev-2 接收的 x 应为 360');

  const p3View = new DataView(dev3Packets[0].buffer, dev3Packets[0].byteOffset, dev3Packets[0].byteLength);
  assert.equal(p3View.getInt32(10, false), 540, 'dev-3 接收的 x 应为 540');

  // 取消 dev-2 注册
  unreg2();
  gc.broadcastControl('dev-1', inputBytes, { width: 1080, height: 2400 });
  assert.equal(dev2Packets.length, 1, '取消注册后 dev-2 不应再收到新包');
  assert.equal(dev3Packets.length, 2, 'dev-3 应收到第 2 条广播消息');

  unreg3();
  gc.setActive(false);
  console.log('PASS: useGroupControl unit test passed successfully');
})();
