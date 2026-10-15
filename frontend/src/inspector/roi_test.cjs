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
        async detectOCR(form) {
          captured = form;
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

  console.log('PASS: All ROI frontend regression tests passed');
  if (hadDocument) global.document = previousDocument;
  else delete global.document;
})();
