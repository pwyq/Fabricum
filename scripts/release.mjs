import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
function run(command, args) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', windowsHide: true })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
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
// Exact source allowlist excludes local art, native packages, executables, and git history.
const sources = [
  '.editorconfig', '.gitattributes', '.gitignore', 'CHANGELOG.md', 'LICENSE', 'README.md', 'VERSION', 'install.sh',
  'docs/README.md', 'docs/configuration.md', 'docs/integration.md',
  'docs/processing.md', 'docs/development.md', 'docs/dependencies.md', 'docs/release.md',
  'front-end/assets.go', '.github/workflows/build.yml', '.github/workflows/commit-message.yml', '.github/workflows/main-policy.yml',
  '.github/workflows/release.yml',
  '.githooks/commit-msg', '.githooks/pre-commit', '.githooks/pre-push',
  'scripts/code/check-file-loc.sh', 'scripts/git/install-hooks.mjs', 'scripts/git/protect-main.cjs',
  'scripts/git/validate-commit-message.cjs', 'scripts/git/validate-pr-title.cjs',
  'scripts/git/validate-commit-range.cjs', 'scripts/git/commit-guard.test.cjs', 'scripts/git/main-guard.test.cjs',
  'go.mod', 'package.json', 'package-lock.json', 'renovate.json',
  'back-end/fabricum.go', 'back-end/fabricum_test.go', 'back-end/cmd/fabricum/main.go',
  'back-end/internal/cli/config.go', 'back-end/internal/cli/config_test.go', 'back-end/internal/cli/export_command.go', 'back-end/internal/cli/export_command_test.go',
  'back-end/internal/editor/config.go', 'back-end/internal/editor/export_handlers.go', 'back-end/internal/editor/handlers_test.go',
  'back-end/internal/editor/lifecycle.go', 'back-end/internal/editor/lifecycle_test.go', 'back-end/internal/editor/run.go',
  'back-end/internal/editor/security.go', 'back-end/internal/editor/security_test.go', 'back-end/internal/editor/server.go',
  'back-end/internal/editor/source_handler.go', 'back-end/internal/editor/source_upload_handler.go',
  'back-end/internal/processing/encoder.go', 'back-end/internal/processing/files.go', 'back-end/internal/processing/processor.go',
  'back-end/internal/processing/processor_test.go', 'back-end/internal/processing/resize.go', 'back-end/internal/processing/types.go',
  'back-end/internal/processing/encoder/encode.mjs', 'scripts/check.mjs', 'scripts/build.mjs', 'scripts/release.mjs',
  'scripts/release/extract-release-notes.cjs', 'scripts/release/release.sh',
  'scripts/release/validate-release-tag.cjs', 'scripts/release/validate-release-tag.test.cjs',
  'front-end/static/api.js', 'front-end/static/app.js', 'front-end/static/app-utils.js', 'front-end/static/ui.js', 'front-end/static/app.css',
  'front-end/static/preview.css', 'front-end/static/crop.js', 'front-end/static/index.html', 'front-end/static/source-selection.js', 'front-end/tests/crop.test.js',
]
run('tar', ['-czf', `bin/${archive}`, ...sources])
const hash = createHash('sha256').update(readFileSync(resolve(root, 'bin', archive))).digest('hex')
writeFileSync(resolve(root, 'bin', `${archive}.sha256`), `${hash}  ${archive}\n`)
console.log(`Created bin/${archive}; no publication or tagging performed.`)
