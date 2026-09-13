import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', windowsHide: true, ...options })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
  return result
}
run(process.execPath, ['scripts/check.mjs'])
const version = readFileSync(resolve(root, 'VERSION'), 'utf8').trim()
const packageMetadata = JSON.parse(readFileSync(resolve(root, 'package.json')))
if (packageMetadata.version !== version) throw new Error('Package and VERSION values differ')
if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(alpha|beta|rc)\.(0|[1-9]\d*))?$/.test(version)) {
  throw new Error('Expected a supported semantic version')
}
mkdirSync(resolve(root, 'bin'), { recursive: true })
const archive = `fabricum-${version}-source.tar.gz`
// Git is the source manifest: ignored dependencies, outputs, and local files
// stay out of the archive, while newly tracked project files are included.
const sources = run('git', ['ls-files', '-z'], { stdio: ['ignore', 'pipe', 'inherit'] }).stdout
if (sources.length === 0) throw new Error('Git reported no tracked source files')
run('tar', ['-czf', `bin/${archive}`, '--null', '--files-from=-'], {
  input: sources,
  stdio: ['pipe', 'inherit', 'inherit'],
})
const hash = createHash('sha256').update(readFileSync(resolve(root, 'bin', archive))).digest('hex')
writeFileSync(resolve(root, 'bin', `${archive}.sha256`), `${hash}  ${archive}\n`)
console.log(`Created bin/${archive}; no publication or tagging performed.`)
