import assert from 'node:assert/strict'
import test from 'node:test'

import {
  PHONE_TIER_SPECS,
  getPhoneTileDimensionClasses,
  computeContainDimensions,
  mapViewportToDeviceCoords,
} from '../src/utils/phoneGrid.ts'

test('PHONE_TIER_SPECS defines uniform dimensions and 2px gap across small, medium, large and fit', () => {
  // small: 200 x 380, gap-[2px]
  assert.equal(PHONE_TIER_SPECS.small.width, 200)
  assert.equal(PHONE_TIER_SPECS.small.height, 380)
  assert.match(PHONE_TIER_SPECS.small.cardClass, /w-\[200px\]/)
  assert.match(PHONE_TIER_SPECS.small.cardClass, /h-\[380px\]/)
  assert.match(PHONE_TIER_SPECS.small.containerClass, /gap-\[2px\]/)

  // medium: 270 x 508, gap-[2px]
  assert.equal(PHONE_TIER_SPECS.medium.width, 270)
  assert.equal(PHONE_TIER_SPECS.medium.height, 508)
  assert.match(PHONE_TIER_SPECS.medium.cardClass, /w-\[270px\]/)
  assert.match(PHONE_TIER_SPECS.medium.cardClass, /h-\[508px\]/)
  assert.match(PHONE_TIER_SPECS.medium.containerClass, /gap-\[2px\]/)

  // large: 348 x 648 (width 348 < 375/390 to guarantee no narrow viewport overflow)
  assert.equal(PHONE_TIER_SPECS.large.width, 348)
  assert.equal(PHONE_TIER_SPECS.large.height, 648)
  assert.ok(PHONE_TIER_SPECS.large.width <= 375, 'large tier card must fit within 375px mobile viewport')
  assert.match(PHONE_TIER_SPECS.large.cardClass, /w-\[348px\]/)
  assert.match(PHONE_TIER_SPECS.large.cardClass, /h-\[648px\]/)
  assert.match(PHONE_TIER_SPECS.large.cardClass, /max-w-full/)
  assert.match(PHONE_TIER_SPECS.large.containerClass, /gap-\[2px\]/)

  // fit: dynamic width/height, locked 2px gap
  assert.equal(PHONE_TIER_SPECS.fit.width, null)
  assert.equal(PHONE_TIER_SPECS.fit.height, null)
  assert.match(PHONE_TIER_SPECS.fit.cardClass, /w-full h-full/)
  assert.match(PHONE_TIER_SPECS.fit.containerClass, /gap-\[2px\]/)
})

test('getPhoneTileDimensionClasses returns 2px gap and exact dimensions for all tiers', () => {
  // fit
  const fitClasses = getPhoneTileDimensionClasses('fit')
  assert.match(fitClasses.containerClass, /gap-\[2px\]/)
  assert.match(fitClasses.cardClass, /w-full h-full/)

  // small
  const smallClasses = getPhoneTileDimensionClasses('small')
  assert.match(smallClasses.containerClass, /gap-\[2px\]/)
  assert.match(smallClasses.cardClass, /w-\[200px\]/)
  assert.match(smallClasses.cardClass, /h-\[380px\]/)

  // medium
  const medClasses = getPhoneTileDimensionClasses('medium')
  assert.match(medClasses.containerClass, /gap-\[2px\]/)
  assert.match(medClasses.cardClass, /w-\[270px\]/)
  assert.match(medClasses.cardClass, /h-\[508px\]/)

  // large
  const largeClasses = getPhoneTileDimensionClasses('large')
  assert.match(largeClasses.containerClass, /gap-\[2px\]/)
  assert.match(largeClasses.cardClass, /w-\[348px\]/)
  assert.match(largeClasses.cardClass, /h-\[648px\]/)
  assert.match(largeClasses.cardClass, /max-w-full/)
})

test('computeContainDimensions: portrait phone fits proportionally with horizontal black bars', () => {
  // 设备分辨率 1080x2400 (9:20 细长屏)，可用视口 200x352
  const res = computeContainDimensions(1080, 2400, 200, 352)
  // 高度占满 352，宽度 = 352 * (1080/2400) = 158.4 -> 158
  assert.equal(res.height, 352)
  assert.equal(res.width, 158)
  assert.equal(res.offsetY, 0)
  assert.equal(res.offsetX, 21) // (200 - 158) / 2 = 21 居中黑边
  assert.ok(res.width <= 200)
  assert.ok(res.height <= 352)
})

test('computeContainDimensions: landscape device fits proportionally with vertical black bars, never stretching cell height', () => {
  // 横屏手机 2400x1080 (20:9 横屏)，可用视口 200x352
  const res = computeContainDimensions(2400, 1080, 200, 352)
  // 宽度占满 200，高度 = 200 * (1080/2400) = 90
  assert.equal(res.width, 200)
  assert.equal(res.height, 90)
  assert.equal(res.offsetX, 0)
  assert.equal(res.offsetY, 131) // (352 - 90) / 2 = 131 上下居中黑边
  assert.ok(res.width <= 200)
  assert.ok(res.height <= 352)
})

test('mapViewportToDeviceCoords maps accurately when clicking inside canvas and rejects black bars', () => {
  const canvasRect = { left: 100, top: 50, width: 200, height: 400 }
  const deviceW = 1080
  const deviceH = 2160

  // 1. 真实画面中心点击 (200, 250) -> x 相对 left 100 (50%), y 相对 top 200 (50%)
  const center = mapViewportToDeviceCoords(200, 250, canvasRect, deviceW, deviceH)
  assert.ok(center)
  assert.equal(center.x, 540)
  assert.equal(center.y, 1080)

  // 2. 左侧黑边点击 (90 < 100) -> 必须被拒绝返回 null
  const leftBlackBar = mapViewportToDeviceCoords(90, 250, canvasRect, deviceW, deviceH)
  assert.equal(leftBlackBar, null)

  // 3. 右侧黑边点击 (310 > 300) -> 必须被拒绝返回 null
  const rightBlackBar = mapViewportToDeviceCoords(310, 250, canvasRect, deviceW, deviceH)
  assert.equal(rightBlackBar, null)

  // 4. 上侧黑边点击 (40 < 50) -> 必须被拒绝返回 null
  const topBlackBar = mapViewportToDeviceCoords(200, 40, canvasRect, deviceW, deviceH)
  assert.equal(topBlackBar, null)

  // 5. 下侧黑边点击 (460 > 450) -> 必须被拒绝返回 null
  const bottomBlackBar = mapViewportToDeviceCoords(200, 460, canvasRect, deviceW, deviceH)
  assert.equal(bottomBlackBar, null)

  // 6. 拖拽模式 (strict = false): 当指针滑出 canvas 时坐标正确截断至设备边缘，不丢失拖拽释放
  const clampedDragLeft = mapViewportToDeviceCoords(50, 250, canvasRect, deviceW, deviceH, false)
  assert.deepEqual(clampedDragLeft, { x: 0, y: 1080 })

  const clampedDragRight = mapViewportToDeviceCoords(350, 250, canvasRect, deviceW, deviceH, false)
  assert.deepEqual(clampedDragRight, { x: 1079, y: 1080 })
})
