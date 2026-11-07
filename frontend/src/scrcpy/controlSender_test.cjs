const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const ts = require('typescript');
const modules = new Map();
function load(file) {
  if (modules.has(file)) return modules.get(file);
  const module = { exports: {} };
  const source = ts.transpileModule(fs.readFileSync(path.join(__dirname, file + '.ts'), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  vm.runInNewContext(source, { module, exports: module.exports, Uint8Array, DataView,
    require(name) { return load(name.replace('./', '')); },
  });
  modules.set(file, module.exports);
  return module.exports;
}
const { ControlSender } = load('controlSender');
const { ControlPacker, ACTION_DOWN, ACTION_MOVE, ACTION_UP, POINTER_MOUSE, POINTER_FINGER_BASE } = load('protocol');
const packer = new ControlPacker(); packer.setDeviceSize(1080, 2400);
const message = (action, x, pointer = POINTER_MOUSE, fromGroupSync = false) => ({
  bytes: packer.touch(action, pointer, x, 100, action === ACTION_UP ? 0 : 1),
  fromGroupSync, size: { width: 1080, height: 2400 },
});
function fixture() {
  let now = 0, buffered = 0, nextId = 0;
  const frames = new Map(), timers = new Map(), sent = [], congestions = [];
  const sender = new ControlSender({
    write: message => sent.push(message), bufferedAmount: () => buffered,
    requestFrame: cb => { const id = ++nextId; frames.set(id, cb); return id; },
    cancelFrame: id => frames.delete(id),
    setTimer: (cb, delay) => { const id = ++nextId; timers.set(id, { cb, at: now + delay }); return id; },
    clearTimer: id => timers.delete(id), now: () => now,
    onCongested: reason => congestions.push(reason),
  });
  const frame = () => { const copy = [...frames]; frames.clear(); copy.forEach(([, cb]) => cb()); };
  const tick = ms => { now += ms; for (const [id, item] of [...timers]) if (item.at <= now) { timers.delete(id); item.cb(); } };
  return { sender, sent, frames, timers, congestions, frame, tick, buffer: value => { buffered = value; } };
}
{
  const f = fixture(); f.sender.send(message(ACTION_DOWN, 0));
  for (let i = 1; i <= 1000; i++) f.sender.send(message(ACTION_MOVE, i));
  assert.equal(f.sent.length, 1, 'DOWN is immediate; raw moves wait for one animation frame');
  assert.equal(f.frames.size, 1, 'a move burst schedules only one frame');
  f.frame(); assert.equal(f.sent.length, 2);
  assert.equal(new DataView(f.sent[1].bytes.buffer).getInt32(10), 1000, 'only the latest coordinate is sent');
  f.sender.send(message(ACTION_MOVE, 1010)); f.sender.send(message(ACTION_UP, 1010));
  assert.deepEqual(f.sent.map(p => p.bytes[1]), [ACTION_DOWN, ACTION_MOVE, ACTION_MOVE, ACTION_UP]);
  f.frame(); assert.equal(f.sent.length, 4, 'UP cancels the delayed duplicate');
}
{
  const f = fixture(); f.sender.send(message(ACTION_DOWN, 0));
  f.sender.send(message(ACTION_MOVE, 20));
  f.sender.send({ bytes: packer.keyCode(0, 3), fromGroupSync: false });
  assert.deepEqual(f.sent.map(p => p.bytes[0]), [2, 2, 0], 'keys remain after prior motion');
  f.sender.reset(); assert.deepEqual(f.sent.slice(-2).map(p => [p.bytes[0], p.bytes[1]]), [[2, 1], [0, 1]], 'reset releases both pointer and key');
  assert.equal(f.frames.size, 0); assert.equal(f.timers.size, 0);
  f.frame(); assert.equal(f.sent.length, 5, 'reset has no late write');
}
{
  const f = fixture();
  f.sender.send(message(ACTION_DOWN, 0)); f.sender.send(message(ACTION_DOWN, 5, POINTER_FINGER_BASE));
  f.sender.send(message(ACTION_MOVE, 10)); f.sender.send(message(ACTION_MOVE, 15, POINTER_FINGER_BASE));
  f.frame(); assert.deepEqual(f.sent.slice(2).map(p => new DataView(p.bytes.buffer).getInt32(10)), [10, 15], 'pointer identities do not overwrite each other');
}
{
  const f = fixture(); f.sender.send(message(ACTION_DOWN, 0));
  f.buffer(200000); f.sender.send(message(ACTION_MOVE, 20)); f.frame();
  assert.equal(f.sent.length, 1, 'congestion retains only the latest unsent motion');
  for (let i = 0; i < 1000; i++) f.sender.send(message(ACTION_MOVE, i));
  assert.equal(f.timers.size, 1, 'congestion polling stays bounded');
  f.buffer(0); f.tick(16);
  assert.equal(f.sent.length, 2); assert.equal(new DataView(f.sent[1].bytes.buffer).getInt32(10), 999);
}
{
  const f = fixture(); f.sender.send(message(ACTION_DOWN, 0));
  f.buffer(200000); f.sender.send(message(ACTION_MOVE, 20)); f.frame(); f.tick(500);
  assert.equal(f.congestions.length, 1, 'persistent congestion triggers explicit connection recovery');
  assert.equal(f.sent.at(-1).bytes[1], ACTION_UP, 'recovery attempts release before closing');
  assert.equal(f.frames.size, 0); assert.equal(f.timers.size, 0);
  f.frame(); assert.equal(f.sent.length, 2, 'congested moves never replay after recovery');
  f.sender.send(message(ACTION_UP, 20)); assert.equal(f.sent.length, 2, 'failed epoch does not accept more input');
  f.sender.reset(); f.buffer(0); f.sender.send(message(ACTION_DOWN, 10)); assert.equal(f.sent.length, 3, 'next connection starts a new epoch');
}
{
  const f = fixture(); f.sender.send(message(ACTION_DOWN, 0));
  f.buffer(200000); f.sender.send(message(ACTION_MOVE, 20)); f.sender.send(message(ACTION_UP, 20));
  assert.equal(f.congestions.length, 1, 'a release behind an overfull socket closes instead of silently queueing');
  assert.equal(f.sent.at(-1).bytes[1], ACTION_UP);
}
{
  const f = fixture(); const fromGroup = message(ACTION_MOVE, 30, POINTER_MOUSE, true);
  f.sender.send(fromGroup); assert.equal(f.sent[0], fromGroup, 'already coalesced group motion has no second frame delay');
  assert.equal(f.frames.size, 0);
  f.sender.send(message(ACTION_DOWN, 0)); f.sender.send(message(ACTION_MOVE, 30));
  f.sender.releaseLocal(); assert.equal(f.sent.at(-1).bytes[1], ACTION_UP);
  assert.equal(f.frames.size, 0, 'blur drops unsent local motion');
}
{
  const f = fixture(); f.buffer(100); f.sender.send(message(ACTION_DOWN, 0));
  assert.equal(f.timers.size, 1, 'even a small backlog is monitored');
  f.tick(500); assert.equal(f.congestions.length, 1, 'low-volume stalled sockets do not wait for kilobytes of motion');
}
{
  const f = fixture(); f.sender.send(message(ACTION_DOWN, 0));
  f.sender.send(message(ACTION_MOVE, 20)); const stale = [...f.frames.values()][0];
  f.sender.reset(); f.sender.send(message(ACTION_DOWN, 1)); f.sender.send(message(ACTION_MOVE, 30));
  const count = f.sent.length; stale(); assert.equal(f.sent.length, count, 'late callbacks from the old epoch cannot flush new input');
  f.frame(); assert.equal(f.sent.length, count + 1);
}
{
  const f = fixture();
  f.sender.send({ bytes: new Uint8Array(262144), fromGroupSync: false });
  assert.equal(f.sent.length, 1, 'a single large text/clipboard payload is not truncated or size-rejected');
}
console.log('PASS: input move coalescing, barriers, group delivery, cancellation, bounded congestion and held input release');
