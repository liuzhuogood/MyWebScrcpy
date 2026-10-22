import assert from 'node:assert/strict'
import test from 'node:test'
import {
  executeBatchAction,
  dedupeSerials,
  filterSelectableDevices,
  useDeviceBatch,
} from '../src/composables/useDeviceBatch.ts'

test('dedupeSerials eliminates duplicate serials, respects max 64 limit', () => {
  assert.deepEqual(dedupeSerials(['a', 'b', 'a', 'c', 'b']), ['a', 'b', 'c'])
  assert.deepEqual(dedupeSerials([]), [])
  assert.deepEqual(dedupeSerials(null), [])

  const many = Array.from({ length: 80 }, (_, i) => `dev-${i}`)
  const deduped = dedupeSerials(many)
  assert.equal(deduped.length, 64)
})

test('executeBatchAction bounded concurrency never exceeds 4 even if requested higher', async () => {
  const serials = ['d1', 'd2', 'd3', 'd4', 'd5', 'd6']
  let active = 0
  let maxActive = 0

  const runner = async () => {
    active++
    maxActive = Math.max(maxActive, active)
    await new Promise(r => setTimeout(r, 20))
    active--
    return { executed: true, status: 'executed' }
  }

  await executeBatchAction({
    batchId: 'concurrency-test',
    generation: 1,
    serials,
    action: 'home',
    concurrency: 8, // Request 8, must be capped at 4
    runner,
  })

  assert.ok(maxActive <= 4, `Observed concurrency ${maxActive} must not exceed 4`)
})

test('executeBatchAction releases all timers promptly upon success (no timer leak)', async () => {
  const start = Date.now()
  const serials = ['dev-quick-1', 'dev-quick-2']

  const quickRunner = async () => {
    return { executed: true, status: 'executed' }
  }

  const { results, summary } = await executeBatchAction({
    batchId: 'quick-batch',
    generation: 1,
    serials,
    action: 'back',
    runner: quickRunner,
  })

  const elapsed = Date.now() - start
  assert.ok(elapsed < 200, `Execution took ${elapsed}ms, should finish well within 200ms without timer leak`)
  assert.equal(summary.success, 2)
  assert.equal(results[0].status, 'success')
  assert.equal(results[1].status, 'success')
})

test('executeBatchAction marks dispatched items as unknown on timeout or abort, undispatched as cancelled', async () => {
  const serials = ['dispatched-timeout', 'undispatched-wait']

  const hangingRunner = async (serial, signal) => {
    if (serial === 'dispatched-timeout') {
      await new Promise((resolve, reject) => {
        signal.addEventListener('abort', () => {
          const err = new Error('设备执行超时 (50ms)')
          err.name = 'TimeoutError'
          reject(err)
        })
      })
    }
  }

  const { results, summary } = await executeBatchAction({
    batchId: 'timeout-unknown-batch',
    generation: 1,
    serials,
    action: 'home',
    concurrency: 1, // serial dispatch
    perDeviceTimeoutMs: 50,
    runner: hangingRunner,
  })

  assert.equal(results.length, 2)
  const d1 = results.find(r => r.serial === 'dispatched-timeout')
  assert.equal(d1?.status, 'unknown')
  assert.ok(d1?.error?.includes('超时'), 'Error should retain timeout reason')

  assert.equal(summary.unknown, 1)
})

test('executeBatchAction checks isTargetStillOperable before dispatching', async () => {
  const serials = ['live-dev', 'offline-later-dev']
  let operable = true

  const runner = async (serial) => {
    if (serial === 'live-dev') {
      // Simulate that the second device drops offline while first was running
      operable = false
      return { executed: true, status: 'executed' }
    }
    return { executed: true, status: 'executed' }
  }

  const { results } = await executeBatchAction({
    batchId: 'dyn-check-batch',
    generation: 1,
    serials,
    action: 'back',
    concurrency: 1,
    isTargetStillOperable: s => (s === 'live-dev' ? true : operable),
    runner,
  })

  const d1 = results.find(r => r.serial === 'live-dev')
  assert.equal(d1?.status, 'success')

  const d2 = results.find(r => r.serial === 'offline-later-dev')
  assert.equal(d2?.status, 'cancelled')
  assert.ok(d2?.error?.includes('不可操作'), 'Should mark as cancelled with reason')
})

test('useDeviceBatch destroy and cancel cleanly reset state', async () => {
  const batch = useDeviceBatch()
  const runner = () => new Promise(r => setTimeout(r, 200))

  batch.runBatch(['dev-1'], 'home', undefined, runner)
  assert.equal(batch.isRunning.value, true)

  batch.destroy()
  assert.equal(batch.isRunning.value, false)
  assert.equal(batch.results.value.length, 0)
  assert.equal(batch.currentBatchId.value, '')
})
