const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert');
const { createRequire } = require('node:module');

const frontendDir = path.resolve(__dirname, '../..');
const nativeRequire = createRequire(path.join(frontendDir, 'package.json'));
const ts = nativeRequire('typescript');
const vue = nativeRequire('vue');
const root = __dirname;
const cache = new Map();
let captured;
let capturedSignal;
let uploadOverride;

const vueFixture = { ...vue, provide() {}, onUnmounted() {} };

function load(file) {
  if (cache.has(file)) return cache.get(file);
  const js = ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const module = { exports: {} };
  cache.set(file, module.exports);
  const requireFixture = (name) => {
    if (name === 'vue') return vueFixture;
    if (name === '@/api/ui') return {};
    if (name === '@/api/ocr') {
      return {
        async detectOCR(form, signal) {
          captured = form;
          capturedSignal = signal;
          if (uploadOverride) return uploadOverride(form, signal);
          return {
            status: 'matched',
            matched: true,
            text: 'fixture',
            boxes: [{ text: 'fixture', confidence: 0.99, x: 0.2, y: 0.3, w: 0.2, h: 0.2 }],
            engine: 'fixture',
            elapsed_ms: 10,
          };
        },
      };
    }
    if (name.startsWith('./')) return load(path.resolve(path.dirname(file), name + '.ts'));
    throw new Error('unexpected import ' + name);
  };
  vm.runInThisContext('(function(exports,require,module){' + js + '\n})', { filename: file })(
    module.exports,
    requireFixture,
    module
  );
  return module.exports;
}

(async () => {
  const hadDocument = Object.hasOwn(global, 'document');
  const previousDocument = global.document;
  let drawnRegion;
  global.document = {
    createElement(tag) {
      assert.equal(tag, 'canvas');
      return {
        width: 0,
        height: 0,
        getContext() {
          return {
            drawImage(source, sx, sy, sw, sh) {
              drawnRegion = { source, sx, sy, sw, sh };
            },
          };
        },
        toBlob(callback, type) {
          const { sx, sy, sw, sh } = drawnRegion;
          callback(new Blob([`CROPPED_${sw}x${sh}_FROM_${sx}_${sy}`], { type }));
        },
      };
    },
  };

  // Test 1: Selected element is cropped and not uploaded as full frame
  let fullFrameCalls = 0;
  const canvas = {
    width: 100,
    height: 50,
    toBlob(callback) {
      fullFrameCalls++;
      callback(new Blob(['FULL_FRAME_100x50'], { type: 'image/png' }));
    },
  };
  const inspectorModule = load(root + '/useInspector.ts');
  const inspector = inspectorModule.provideInspector('fixture-device', () => canvas);

  inspector.selectedNode.value = {
    getAttribute(name) {
      return name === 'bounds' ? '[10,10][50,30]' : '';
    },
  };

  await inspector.runOcr();
  assert.equal(fullFrameCalls, 0, 'Source canvas.toBlob should not be called when ROI is cropped');
  const bbox = JSON.parse(captured.get('bbox'));
  assert.deepStrictEqual(bbox, [0.1, 0.2, 0.4, 0.4], 'Normalized ROI bbox should match element bounds on full canvas');
  const uploaded = await captured.get('file').text();
  assert.equal(uploaded, 'CROPPED_40x20_FROM_10_10', 'Uploaded content must be cropped region 40x20');
  assert.equal(captured.get('file').name, 'crop.png');
  assert.equal(drawnRegion.sw, 40);
  assert.equal(drawnRegion.sh, 20);

  // Test 2: Full screen fallback when no element selected
  inspector.selectedNode.value = null;
  await inspector.runOcr();
  assert.equal(fullFrameCalls, 1, 'Source canvas.toBlob should be called for full screen OCR');
  const fullBBox = JSON.parse(captured.get('bbox'));
  assert.deepStrictEqual(fullBBox, [0, 0, 1, 1], 'Full screen bbox should be [0,0,1,1]');
  const fullUploaded = await captured.get('file').text();
  assert.equal(fullUploaded, 'FULL_FRAME_100x50');
  assert.equal(captured.get('file').name, 'screen.png');

  // Test 3: Clamping out-of-bounds element
  inspector.selectedNode.value = {
    getAttribute(name) {
      return name === 'bounds' ? '[-20,-10][150,80]' : '';
    },
  };
  await inspector.runOcr();
  const clampedBBox = JSON.parse(captured.get('bbox'));
  assert(clampedBBox[0] >= 0 && clampedBBox[0] <= 1, 'BBox X should be within [0,1]');
  assert(clampedBBox[1] >= 0 && clampedBBox[1] <= 1, 'BBox Y should be within [0,1]');
  assert(clampedBBox[2] > 0 && clampedBBox[2] <= 1, 'BBox W should be within (0,1]');
  assert(clampedBBox[3] > 0 && clampedBBox[3] <= 1, 'BBox H should be within (0,1]');

  inspector.selectedNode.value = {
    getAttribute(name) {
      return name === 'bounds' ? '[200,60][220,80]' : '';
    },
  };
  await inspector.runOcr();
  assert.equal(fullFrameCalls, 1, 'An offscreen selected element must not fall back to uploading the full frame');
  assert.match(inspector.ocrError.value, /不在当前画面/);

  // Explicit region selection uses the same crop/mapping contract, and local overlay ownership.
  inspector.selectedNode.value = null;
  inspector.ocr.scope.value = 'region';
  inspector.ocr.region.value = { x: .2, y: .2, w: .3, h: .4 };
  inspector.ocr.engine.value = 'indraocr';
  await inspector.runOcr();
  assert.deepStrictEqual(JSON.parse(captured.get('bbox')), [.2, .2, .3, .4]);
  assert.equal(captured.get('engine'), 'indraocr');
  assert.equal(captured.get('mark'), 'false', 'Server marks must not outlive cancelled inspector work');
  assert.equal(await captured.get('file').text(), 'CROPPED_30x20_FROM_20_10');
  assert.equal(inspector.ocr.coordinates(inspector.ocrResult.value.boxes[0]), 'x:20 y:15 w:20 h:10 px');

  // Region selection is a one-shot gesture; later screen gestures cannot replace it.
  canvas.getBoundingClientRect = () => ({ left: 0, top: 0, width: 100, height: 50 });
  canvas.setPointerCapture = () => {};
  let prevented = 0;
  const pointer = (x, y) => ({ clientX: x, clientY: y, button: 0, pointerId: 1, preventDefault() { prevented++; } });
  inspector.ocr.beginRegionSelection();
  assert.equal(inspector.ocr.selecting.value, true);
  inspector.ocr.pointerDown(pointer(20, 10));
  inspector.ocr.pointerUp(pointer(50, 30));
  assert.equal(inspector.ocr.selecting.value, false, 'Finishing the region must immediately release screen input');
  const fixedRegion = JSON.stringify(inspector.ocr.region.value);
  inspector.ocr.pointerDown(pointer(80, 40));
  inspector.ocr.pointerMove(pointer(90, 45));
  inspector.ocr.pointerUp(pointer(90, 45));
  assert.equal(JSON.stringify(inspector.ocr.region.value), fixedRegion, 'Ordinary device gestures must not alter the OCR region');
  assert.equal(prevented, 1, 'Only the region-selection gesture may prevent device input');
  const guard = load(root + '/guard.ts');
  guard.setInspector(true, 'ocr');
  assert.equal(guard.inspectorConsumesInput(), false, 'Opening the inspector alone must not block input');
  guard.inspectionMode.value = true;
  assert.equal(guard.inspectorConsumesInput(), true);
  guard.setInspector(false);
  assert.equal(guard.inspectorConsumesInput(), false, 'Closing the inspector must release selection input');
  assert.equal(guard.inspectionMode.value, false);
  for (const tab of ['color', 'xml', 'ocr', 'page', 'log']) {
    guard.setInspector(true, tab);
    assert.equal(guard.inspectorConsumesInput(), false, `Opening ${tab} must preserve device control`);
  }

  const response = { status: 'matched', matched: true, text: 'late result', boxes: [], engine: 'indraocr', elapsed_ms: 10 };
  const tick = () => new Promise(resolve => setImmediate(resolve));
  let finish;
  uploadOverride = () => new Promise(resolve => { finish = resolve; });
  inspector.ocr.scope.value = 'screen';
  const cancelled = inspector.runOcr();
  await tick();
  inspector.clearOcr();
  assert.equal(capturedSignal.aborted, true);
  finish(response);
  await cancelled;
  assert.equal(inspector.ocrResult.value, null, 'An uncooperative old response must not overwrite cleared state');

  const resized = inspector.runOcr();
  await tick();
  canvas.width = 120;
  finish(response);
  await resized;
  assert.equal(inspector.ocrResult.value, null, 'Rotation during recognition must reject old geometry');
  canvas.width = 100;

  // Cancellation also releases an unfinished PNG encoding, before any HTTP request.
  const toBlob = canvas.toBlob;
  canvas.toBlob = () => {};
  captured = null;
  const encoding = inspector.runOcr();
  inspector.ocr.stop();
  await encoding;
  assert.equal(captured, null);
  canvas.toBlob = toBlob;

  // Only one continuous request is in flight; stopping prevents subsequent schedules.
  inspector.ocr.continuous.value = true;
  inspector.ocr.beginRegionSelection();
  inspector.ocr.pointerDown(pointer(20, 10));
  inspector.ocr.pointerUp(pointer(50, 30));
  let calls = 0;
  const continuousBoxes = [];
  uploadOverride = form => { calls++; continuousBoxes.push(JSON.parse(form.get('bbox'))); return new Promise(resolve => { finish = resolve; }); };
  const continuous = inspector.runOcr();
  await tick();
  await new Promise(resolve => setTimeout(resolve, 550));
  assert.equal(calls, 1);
  inspector.ocr.pointerDown(pointer(75, 35));
  inspector.ocr.pointerUp(pointer(90, 45));
  finish(response);
  await continuous;
  await new Promise(resolve => setTimeout(resolve, 40));
  assert.equal(calls, 2);
  assert.deepStrictEqual(continuousBoxes, [[.2, .2, .3, .4], [.2, .2, .3, .4]], 'Continuous OCR must retain the selected region across screen operations');
  inspector.ocr.panelHidden();
  inspector.closeInspector();
  guard.setInspector(false);
  assert.equal(inspector.ocr.running.value, true, 'Closing the inspector must not stop continuous OCR');
  assert.equal(capturedSignal.aborted, false, 'Hidden panels must not abort continuous recognition');
  finish(response);
  await new Promise(resolve => setTimeout(resolve, 550));
  assert.equal(calls, 3, 'Continuous OCR must schedule another request with the panel closed');
  assert.deepStrictEqual(continuousBoxes[2], [.2, .2, .3, .4]);
  inspector.ocr.stop();
  finish(response);
  await new Promise(resolve => setTimeout(resolve, 550));
  assert.equal(calls, 3, 'Stopping must cancel the next continuous request');
  assert.equal(inspector.ocr.running.value, false);

  // A continuous element ROI belongs to the task, not the XML tab selection.
  const elementLoop = inspectorModule.provideInspector('element-loop', () => canvas);
  elementLoop.selectedNode.value = { getAttribute: name => name === 'bounds' ? '[10,10][50,30]' : '' };
  elementLoop.ocr.continuous.value = true;
  const elementBoxes = [];
  uploadOverride = form => { elementBoxes.push(JSON.parse(form.get('bbox'))); return new Promise(resolve => { finish = resolve; }); };
  const elementPending = elementLoop.runOcr();
  await tick();
  elementLoop.selectedNode.value = null;
  elementLoop.ocr.panelHidden();
  elementLoop.closeInspector();
  finish(response);
  await elementPending;
  await new Promise(resolve => setTimeout(resolve, 550));
  assert.deepStrictEqual(elementBoxes, [[.1, .2, .4, .4], [.1, .2, .4, .4]], 'Changing XML selection must not redirect an active continuous task');
  elementLoop.ocr.invalidate();
  assert.equal(capturedSignal.aborted, true, 'Disconnecting or destroying the player must abort background OCR');
  finish(response);
  await tick();
  assert.equal(elementLoop.ocr.running.value, false);

  // A slow result from an advanced video frame cannot restore stale boxes.
  let frameVersion = 0;
  const fresh = inspectorModule.provideInspector('fresh-device', () => canvas, () => frameVersion);
  const now = Date.now;
  let clock = 1000;
  Date.now = () => clock;
  const stale = fresh.runOcr();
  await tick();
  frameVersion++;
  clock += 650;
  finish(response);
  await stale;
  Date.now = now;
  assert.equal(fresh.ocrResult.value, null);
  assert.match(fresh.ocrError.value, /画面已变化/);
  uploadOverride = async () => { throw new Error('connection refused at internal endpoint'); };
  await fresh.runOcr();
  assert.match(fresh.ocrError.value, /请检查 OCR 服务/);
  assert(!fresh.ocrError.value.includes('internal endpoint'), 'Transport details belong in logs, not the product UI');
  const notReady = inspectorModule.provideInspector('connecting-device', () => canvas, () => 0, () => false);
  captured = null;
  await notReady.runOcr();
  assert.equal(captured, null, 'A sized but not-yet-decoded canvas is not a valid source frame');
  assert.match(notReady.ocrError.value, /未就绪/);
  notReady.ocr.stop();
  fresh.ocr.stop();
  inspector.ocr.stop();

  console.log('PASS: All ROI frontend regression tests passed');
  if (hadDocument) global.document = previousDocument;
  else delete global.document;
})();
