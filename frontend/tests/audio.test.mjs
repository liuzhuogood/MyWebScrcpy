import assert from 'node:assert/strict'
import test from 'node:test'
import { DeviceAudio } from '../src/scrcpy/audio.ts'

test('local playback gain follows volume, mute and unmute without device commands', async t => {
  const gain = { gain: { value: 0 }, connect() {} }
  class Context {
    state = 'running'
    destination = {}
    createGain() { return gain }
    async resume() {}
    async close() {}
  }
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'AudioContext')
  Object.defineProperty(globalThis, 'AudioContext', { configurable: true, value: Context })
  t.after(() => {
    if (previous) Object.defineProperty(globalThis, 'AudioContext', previous)
    else delete globalThis.AudioContext
  })
  const audio = new DeviceAudio()
  audio.status = 'ready'
  await audio.setVolume(0.35)
  assert.equal(gain.gain.value, 0.35)
  assert.equal(audio.muted, false)
  await audio.toggle()
  assert.equal(gain.gain.value, 0)
  await audio.toggle()
  assert.equal(gain.gain.value, 0.35)
  await audio.setVolume(0)
  assert.equal(audio.muted, true)
  await audio.toggle()
  assert.equal(gain.gain.value, 1)
  await audio.setVolume(2)
  assert.equal(audio.volume, 1)
  await audio.setVolume(NaN)
  assert.equal(audio.volume, 1)
  audio.close()
})
