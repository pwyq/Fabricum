const assert = require('node:assert/strict')
const { execFileSync, spawnSync } = require('node:child_process')
const { mkdtempSync, rmSync } = require('node:fs')
const { join } = require('node:path')
const { tmpdir } = require('node:os')
const test = require('node:test')
const {
  isMainBranch,
  isMergedPullRequest,
  isProtectedPush,
  parsePushInput,
} = require('./protect-main.cjs')

test('main branch policy identifies protected commit and push targets', () => {
  assert.equal(isMainBranch('main'), true)
  assert.equal(isMainBranch('codex/feature'), false)
  assert.equal(isProtectedPush({ localRef: 'refs/heads/main', remoteRef: 'refs/heads/feature' }), true)
  assert.equal(isProtectedPush({ localRef: 'refs/heads/feature', remoteRef: 'refs/heads/main' }), true)
  assert.equal(isProtectedPush({ localRef: 'refs/heads/feature', remoteRef: 'refs/heads/feature' }), false)
})

test('main branch policy only accepts merged pull requests targeting this repository', () => {
  const merged = { merged_at: '2026-09-12T00:00:00Z', base: { ref: 'main', repo: { full_name: 'pwyq/Fabricum' } } }
  assert.equal(isMergedPullRequest(merged, 'pwyq/Fabricum'), true)
  assert.equal(isMergedPullRequest({ ...merged, merged_at: null }, 'pwyq/Fabricum'), false)
  assert.equal(isMergedPullRequest({ ...merged, base: { ...merged.base, ref: 'develop' } }, 'pwyq/Fabricum'), false)
  assert.equal(isMergedPullRequest(merged, 'other/project'), false)
})

test('pre-push guard rejects main refs and accepts feature refs', () => {
  const script = join(__dirname, 'protect-main.cjs')
  const localSha = 'a'.repeat(40)
  const remoteSha = 'b'.repeat(40)
  const run = input => spawnSync(process.execPath, [script, 'push'], {
    input,
    encoding: 'utf8',
    windowsHide: true,
  })

  assert.equal(run(`refs/heads/feature ${localSha} refs/heads/feature ${remoteSha}\n`).status, 0)
  assert.equal(run(`refs/heads/feature ${localSha} refs/heads/main ${remoteSha}\n`).status, 1)
  assert.equal(run(`refs/heads/main ${localSha} refs/heads/feature ${remoteSha}\n`).status, 1)
  assert.throws(() => parsePushInput('malformed line'), /Malformed pre-push input/)
})

test('pre-commit guard rejects a repository checked out on main', t => {
  const directory = mkdtempSync(join(tmpdir(), 'fabricum-main-guard-'))
  t.after(() => rmSync(directory, { recursive: true, force: true }))
  execFileSync('git', ['init', '--quiet', '--initial-branch=main'], { cwd: directory })

  const script = join(__dirname, 'protect-main.cjs')
  const run = () => spawnSync(process.execPath, [script, 'commit'], {
    cwd: directory,
    encoding: 'utf8',
    windowsHide: true,
  })

  assert.equal(run().status, 1)
  execFileSync('git', ['switch', '--quiet', '-c', 'feature'], { cwd: directory })
  assert.equal(run().status, 0)
})
