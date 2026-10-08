const { test } = require('node:test')
const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')

test('Windows PNG build uses locked source and static runtime without host flag overrides', async () => {
  const { installPNGOptimizer } = await import('./png-optimizer.mjs')
  const pinned = JSON.parse(readFileSync('native/versions.json')).oxipng
  const calls = []
  const services = Object.fromEntries(['download', 'verify', 'extract', 'run', 'copyExecutables'].map(name => [name, (...args) => calls.push({ name, args })]))
  const previous = process.env.CARGO_ENCODED_RUSTFLAGS
  process.env.CARGO_ENCODED_RUSTFLAGS = '-Ctarget-feature=-crt-static'
  try {
    await installPNGOptimizer({ ...services, pinned, workspace: 'workspace', platform: 'win32', arch: 'x64' })
  } finally {
    if (previous === undefined) delete process.env.CARGO_ENCODED_RUSTFLAGS
    else process.env.CARGO_ENCODED_RUSTFLAGS = previous
  }
  assert.equal(calls[0].args[0], pinned.source)
  assert.equal(calls[1].args[1], pinned.sourceSha256)
  const command = calls.find(call => call.name === 'run')
  assert.equal(command.args[0], 'cargo')
  assert.ok(command.args[1].includes('--locked'))
  assert.ok(command.args[1].includes(pinned.windows.target))
  assert.match(command.args[3].RUSTFLAGS, /\+crt-static/)
  assert.equal(command.args[3].RUSTUP_TOOLCHAIN, pinned.windows.toolchain)
  assert.equal(command.args[3].CARGO_ENCODED_RUSTFLAGS, undefined)
  assert.equal(calls.at(-1).name, 'copyExecutables')
})

test('Linux PNG installation verifies the pinned static musl artifact', async () => {
  const { installPNGOptimizer } = await import('./png-optimizer.mjs')
  const pinned = JSON.parse(readFileSync('native/versions.json')).oxipng
  const calls = []
  const services = Object.fromEntries(['download', 'verify', 'extract', 'run', 'copyExecutables'].map(name => [name, (...args) => calls.push({ name, args })]))
  await installPNGOptimizer({ ...services, pinned, workspace: 'workspace', platform: 'linux', arch: 'x64' })
  assert.equal(calls[0].args[0], pinned.linux.artifact)
  assert.equal(calls[1].args[1], pinned.linux.sha256)
  assert.ok(!calls.some(call => call.name === 'run'))
  await assert.rejects(installPNGOptimizer({ ...services, pinned, workspace: 'workspace', platform: 'linux', arch: 'arm64' }), /require x64/)
})
