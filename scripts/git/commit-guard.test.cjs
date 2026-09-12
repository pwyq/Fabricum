const assert = require('node:assert/strict')
const { execFileSync, spawnSync } = require('node:child_process')
const { mkdtempSync, rmSync, writeFileSync } = require('node:fs')
const { tmpdir } = require('node:os')
const { join } = require('node:path')
const test = require('node:test')
const { isValidCommitSubject } = require('./validate-commit-message.cjs')

test('commit and PR subjects require the supported type, message, and issue number', () => {
  for (const subject of ['feat: add export (#1)', 'fix: preserve alpha (#123)', 'ci: check builds (#5)']) {
    assert.equal(isValidCommitSubject(subject), true)
  }
  for (const subject of ['', 'add export', 'feat: add export', 'feat(ui): add export (#1)',
    'feat: add export (#0)', 'unknown: add export (#1)', 'feat: add export (#1) trailing']) {
    assert.equal(isValidCommitSubject(subject), false, subject)
  }
  const script = join(__dirname, 'validate-pr-title.cjs')
  const run = title => spawnSync(process.execPath, [script], {
    env: { ...process.env, PR_TITLE: title }, encoding: 'utf8', windowsHide: true,
  })
  assert.equal(run('feat: add export (#1)').status, 0)
  assert.equal(run('missing type').status, 1)
})

test('commit file and PR range checks reject invalid subjects and exclude base history', t => {
  const directory = mkdtempSync(join(tmpdir(), 'fabricum-commit-guard-'))
  t.after(() => rmSync(directory, { recursive: true, force: true }))
  const options = {
    cwd: directory, encoding: 'utf8', windowsHide: true,
    env: { ...process.env, GIT_AUTHOR_NAME: 'Test', GIT_AUTHOR_EMAIL: 'test@example.invalid',
      GIT_COMMITTER_NAME: 'Test', GIT_COMMITTER_EMAIL: 'test@example.invalid' },
  }
  const git = args => execFileSync('git', args, options).trim()
  git(['init', '--quiet'])
  const commit = subject => {
    git(['-c', 'commit.gpgsign=false', '-c', 'core.hooksPath=/dev/null', 'commit', '--quiet', '--allow-empty', '-m', subject])
    return git(['rev-parse', 'HEAD'])
  }
  const base = commit('existing base history')
  const good = commit('feat: add export (#1)')
  const invalid = commit('missing required format')
  const script = join(__dirname, 'validate-commit-range.cjs')
  const run = args => spawnSync(process.execPath, [script, ...args], options)
  assert.equal(run([base, good]).status, 0)
  assert.equal(run([base, invalid]).status, 1)
  assert.equal(run([]).status, 1)
  assert.equal(run(['--all', good]).status, 1)
  const file = join(directory, 'message.txt')
  writeFileSync(file, '# comment\n\nfix: keep image data (#2)\n\nBody text.\n')
  const check = () => spawnSync(process.execPath, [join(__dirname, 'validate-commit-message.cjs'), file], options)
  assert.equal(check().status, 0)
  writeFileSync(file, 'invalid subject\n')
  assert.equal(check().status, 1)
})
