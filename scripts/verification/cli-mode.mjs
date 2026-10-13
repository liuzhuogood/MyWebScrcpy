import { mkdtemp, copyFile, writeFile, rm } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'

const directory = await mkdtemp(path.join(os.tmpdir(), 'mywebscrcpy-cli-test-'))
try {
  const suffix = process.platform === 'win32' ? '.exe' : ''
  const executable = path.join(directory, 'mywebscrcpy-cli' + suffix)
  await copyFile(path.resolve(process.argv[2]), executable)
  await writeFile(path.join(directory, 'mywebscrcpy-gui' + suffix), 'unusable GUI shell')
  const service = spawnSync(executable, ['--print-mode'], { encoding: 'utf8' })
  assert.equal(service.status, 0, service.stderr)
  assert.equal(service.stdout, 'service\n')
  const gui = spawnSync(executable, ['--gui', '--print-mode'], { encoding: 'utf8' })
  assert.notEqual(gui.status, 0)
  assert.match(gui.stderr, /CLI 版本仅运行服务/)
  console.log('PASS: CLI defaults to service beside GUI and rejects GUI mode')
} finally {
  await rm(directory, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 })
}
