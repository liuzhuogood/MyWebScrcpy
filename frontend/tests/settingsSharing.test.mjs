import test from 'node:test'
import assert from 'node:assert/strict'
import { effectScope } from 'vue'
import { usePlayerSettings } from '../src/composables/usePlayerSettings.ts'

// VueUse intentionally isolates SSR callers. The real-browser sharing contract
// is exercised by scripts/verification/ui-fix-3.mjs.
test('SSR preference callers remain isolated between scopes', () => {
  const first = effectScope(), second = effectScope()
  const a = first.run(() => usePlayerSettings()), b = second.run(() => usePlayerSettings())
  assert.notEqual(a.bitrate, b.bitrate)
  a.maxFps.value = 15
  assert.equal(b.maxFps.value, 30)
  first.stop()
  second.stop()
})
