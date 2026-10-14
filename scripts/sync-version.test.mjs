import { test } from 'node:test'
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const script = readFileSync(fileURLToPath(new URL('./sync-version.mjs', import.meta.url)), 'utf8')
for (const newline of ['\n', '\r\n']) {
  test(`version synchronization preserves ${newline === '\n' ? 'LF' : 'CRLF'}`, t => {
    const root = mkdtempSync(path.join(os.tmpdir(), 'mywebscrcpy-version-'))
    t.after(() => rmSync(root, { recursive: true, force: true }))
    const files = {
      VERSION: 'v1.20.0\n',
      'scripts/sync-version.mjs': script,
      'frontend/package.json': '{\n  "version": "1.20.0"\n}\n',
      'desktop/package.json': '{\n  "version": "1.20.0"\n}\n',
      'desktop/Cargo.toml': '[package]\nname = "mywebscrcpy-gui"\nversion = "1.20.0"\n',
      'desktop/Cargo.lock': '[[package]]\nname = "mywebscrcpy-gui"\nversion = "1.20.0"\n',
      'desktop/tauri.conf.json': JSON.stringify({ version: '1.20.0', plugins: { updater: { pubkey: 'test-public-key' } } }, null, 2) + '\n',
      'desktop/updater.pub': 'test-public-key\n',
      'packages/python-sdk/pyproject.toml': '[project]\nname = "mywebscrcpy-sdk"\nversion = "1.20.0"\n',
      'packages/python-sdk/src/mywebscrcpy/__init__.py': '__version__ = "1.20.0"\n',
      'packages/python-sdk/version.go': 'package pythonsdk\nconst Version = "1.20.0"\n',
    }
    for (const [file, contents] of Object.entries(files)) {
      const full = path.join(root, file)
      mkdirSync(path.dirname(full), { recursive: true })
      writeFileSync(full, contents.replace(/\n/g, newline))
    }
    const run = (...args) => spawnSync(process.execPath, ['scripts/sync-version.mjs', ...args], { cwd: root, encoding: 'utf8' })
    assert.equal(run('--check').status, 0, 'matching versions must pass with either line ending')
    writeFileSync(path.join(root, 'VERSION'), `v1.20.1${newline}`)
    assert.notEqual(run('--check').status, 0, 'a real version mismatch must fail')
    const updated = run()
    assert.equal(updated.status, 0, updated.stderr)
    assert.equal(run('--check').status, 0, 'all package and desktop versions must be updated')
    for (const file of ['frontend/package.json', 'desktop/package.json', 'desktop/Cargo.toml', 'desktop/Cargo.lock', 'desktop/tauri.conf.json', 'packages/python-sdk/pyproject.toml', 'packages/python-sdk/src/mywebscrcpy/__init__.py', 'packages/python-sdk/version.go']) {
      const contents = readFileSync(path.join(root, file), 'utf8')
      assert.ok(contents.includes('1.20.1'), file)
      if (newline === '\r\n') assert.ok(!/(?<!\r)\n/.test(contents), `${file} contains bare LF`)
      else assert.ok(!contents.includes('\r'), `${file} contains CRLF`)
    }
  })
}
