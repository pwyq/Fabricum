import { spawnSync } from 'node:child_process'
import { readdirSync, readFileSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
function run(command, args) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', windowsHide: true })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}
const pkg = JSON.parse(readFileSync(resolve(root, 'package.json')))
const releaseVersion = readFileSync(resolve(root, 'VERSION'), 'utf8').trim()
if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(alpha|beta|rc)\.(0|[1-9]\d*))?$/.test(releaseVersion)) {
  throw new Error('VERSION must contain a supported semantic version')
}
if (pkg.version !== releaseVersion) throw new Error('Package and VERSION values differ')
const files = ['front-end', 'back-end', 'back-end/cmd/fabricum'].flatMap(dir => readdirSync(resolve(root,dir)).filter(n => n.endsWith('.go')).map(n => dir+'/'+n))
const formatted = spawnSync('gofmt', ['-l', ...files], { cwd: root, encoding: 'utf8', windowsHide: true })
if (formatted.error) throw formatted.error
if (formatted.status !== 0 || formatted.stdout.trim()) throw new Error('Run gofmt: '+formatted.stdout+formatted.stderr)
for (const directory of ['front-end/static', 'back-end/encoder', 'scripts', 'scripts/git', 'scripts/release', 'front-end/tests']) {
  for (const name of readdirSync(resolve(root,directory)).filter(n => /\.[cm]?js$/.test(n))) run(process.execPath,['--check',directory+'/'+name])
}
run('go',['vet','./...'])
run('go',['test','./...'])
run(process.execPath,['--test','front-end/tests/crop.test.js','scripts/git/commit-guard.test.cjs','scripts/git/main-guard.test.cjs','scripts/release/validate-release-tag.test.cjs'])
run('go',['build','./...'])
if (!readFileSync(resolve(root,'back-end/config.go'),'utf8').includes('Version = "'+releaseVersion+'"')) throw new Error('Go and release versions differ')
console.log('Formatting, syntax, vet, tests, compilation, and version checks passed.')
