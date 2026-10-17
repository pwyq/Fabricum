const { test } = require('node:test')
const assert = require('node:assert/strict')
const { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, rmSync } = require('node:fs')
const { tmpdir } = require('node:os')
const { join } = require('node:path')

test('JPEG installer verifies source and produces static tools on both release platforms', async () => {
  const { installJPEGOptimizer } = await import('./jpeg-optimizer.mjs')
  const pinned = JSON.parse(readFileSync('native/versions.json')).jpegtran
  for (const platform of ['win32', 'linux']) {
    const workspace = mkdtempSync(join(tmpdir(), 'fabricum-jpeg-installer-test-'))
    try {
      const outputDirectory = join(workspace, 'output')
      mkdirSync(outputDirectory)
      const suffix = platform === 'win32' ? '.exe' : ''
      const produced = join(workspace, `jpegtran-static${suffix}`)
      writeFileSync(produced, 'fixture tool')
      const calls = []
      const services = Object.fromEntries(['download', 'verify', 'extract', 'run'].map(name => [name, (...args) => calls.push({ name, args })]))
      await installJPEGOptimizer({ ...services, pinned, workspace, outputDirectory, findFile: async (_directory, name) => { assert.equal(name, `jpegtran-static${suffix}`); return produced }, platform, arch: 'x64' })
      assert.equal(calls[0].args[0], pinned.source)
      assert.equal(calls[1].args[1], pinned.sourceSha256)
      const configure = calls.find(call => call.name === 'run')
      for (const flag of ['-DENABLE_SHARED=OFF', '-DENABLE_STATIC=ON', '-DWITH_CRT_DLL=OFF', '-DWITH_SIMD=OFF', '-DWITH_TURBOJPEG=OFF']) assert.ok(configure.args[1].includes(flag))
      assert.equal(readFileSync(join(outputDirectory, `jpegtran${suffix}`), 'utf8'), 'fixture tool')
      assert.ok(!existsSync(join(outputDirectory, 'jpegtran-static' + suffix)))
      await assert.rejects(installJPEGOptimizer({ ...services, pinned, workspace, outputDirectory, platform, arch: 'arm64' }), /requires Windows or Linux x64/)
    } finally {
      rmSync(workspace, { recursive: true, force: true })
    }
  }
})
