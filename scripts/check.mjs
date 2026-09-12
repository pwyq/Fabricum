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
const files = ['front-end', 'back-end', 'back-end/cmd/fabricum'].flatMap(dir => readdirSync(resolve(root,dir)).filter(n => n.endsWith('.go')).map(n => dir+'/'+n))
const formatted = spawnSync('gofmt', ['-l', ...files], { cwd: root, encoding: 'utf8', windowsHide: true })
if (formatted.error) throw formatted.error
if (formatted.status !== 0 || formatted.stdout.trim()) throw new Error('Run gofmt: '+formatted.stdout+formatted.stderr)
for (const directory of ['front-end/static', 'back-end/encoder', 'scripts', 'scripts/git', 'front-end/tests']) {
  for (const name of readdirSync(resolve(root,directory)).filter(n => /\.[cm]?js$/.test(n))) run(process.execPath,['--check',directory+'/'+name])
}
run('go',['vet','./...'])
run('go',['test','./...'])
run(process.execPath,['--test','front-end/tests/crop.test.js','scripts/git/commit-guard.test.cjs','scripts/git/main-guard.test.cjs'])
run('go',['build','./...'])
const pkg = JSON.parse(readFileSync(resolve(root,'package.json')))
if (!readFileSync(resolve(root,'back-end/config.go'),'utf8').includes('Version = "'+pkg.version+'"')) throw new Error('Go and package versions differ')
console.log('Formatting, syntax, vet, tests, compilation, and version checks passed.')
