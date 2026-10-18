import assert from 'node:assert/strict'
import test from 'node:test'
import { computeMonitorGrid } from '../src/utils/monitorGrid.ts'

test('0 or negative devices returns 0 rows and cols', () => {
  assert.deepEqual(computeMonitorGrid(0), { rows: 0, cols: 0 })
  assert.deepEqual(computeMonitorGrid(-1), { rows: 0, cols: 0 })
})

test('devices <= 4 use single row with exact device count as cols', () => {
  assert.deepEqual(computeMonitorGrid(1), { rows: 1, cols: 1 })
  assert.deepEqual(computeMonitorGrid(2), { rows: 1, cols: 2 })
  assert.deepEqual(computeMonitorGrid(3), { rows: 1, cols: 3 })
  assert.deepEqual(computeMonitorGrid(4), { rows: 1, cols: 4 })
})

test('devices > 4 split into multiple rows (rows >= 2) and capacity >= count', () => {
  for (let n = 5; n <= 16; n++) {
    const grid = computeMonitorGrid(n, 1920, 1080)
    assert.ok(grid.rows >= 2, `expected rows >= 2 for n=${n}, got ${grid.rows}`)
    assert.ok(grid.rows * grid.cols >= n, `expected capacity >= ${n}, got ${grid.rows * grid.cols}`)
  }
})

test('common counts produce expected optimal matrix on 16:9 monitor', () => {
  // 5 devices: 2 rows of 3 (capacity 6)
  assert.deepEqual(computeMonitorGrid(5, 1920, 1080), { rows: 2, cols: 3 })
  // 6 devices: 2 rows of 3 (capacity 6)
  assert.deepEqual(computeMonitorGrid(6, 1920, 1080), { rows: 2, cols: 3 })
  // 7 devices: 2 rows of 4 (capacity 8)
  assert.deepEqual(computeMonitorGrid(7, 1920, 1080), { rows: 2, cols: 4 })
  // 8 devices: 2 rows of 4 (capacity 8)
  assert.deepEqual(computeMonitorGrid(8, 1920, 1080), { rows: 2, cols: 4 })
})

test('vertical container adapts to taller layout', () => {
  // On a vertical screen (1080w x 1920h), 6 devices should stack vertically with more rows
  const grid = computeMonitorGrid(6, 1080, 1920)
  assert.ok(grid.rows >= grid.cols, `expected rows >= cols on vertical monitor, got ${grid.rows}x${grid.cols}`)
  assert.ok(grid.rows * grid.cols >= 6)
})
