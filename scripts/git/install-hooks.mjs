import { chmodSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const root = resolve(fileURLToPath(new URL('../..', import.meta.url)))
const hookDirectory = resolve(root, '.githooks')

if (!existsSync(resolve(root, '.git'))) {
  console.log('Git metadata not found; skipping local hook installation.')
  process.exit(0)
}

const result = spawnSync('git', ['config', '--local', 'core.hooksPath', '.githooks'], {
  cwd: root,
  stdio: 'inherit',
  windowsHide: true,
})
if (result.error) throw result.error
if (result.status !== 0) process.exit(result.status ?? 1)

for (const name of ['commit-msg', 'pre-commit', 'pre-push']) {
  chmodSync(resolve(hookDirectory, name), 0o755)
}

console.log('Installed local Git hooks from .githooks.')
