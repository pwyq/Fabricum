import { createHash } from 'node:crypto'
import { chmodSync, cpSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const expectedRange = { minimum: 15 * 1024 * 1024, maximum: 50 * 1024 * 1024 }

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { stdio: 'inherit', windowsHide: true, ...options })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} failed with status ${result.status}`)
  return result
}

function option(args, name) {
  const index = args.indexOf(name)
  return index < 0 ? '' : args[index + 1] ?? ''
}

function platformName() {
  const operatingSystem = process.platform === 'win32' ? 'windows' : process.platform === 'linux' ? 'linux' : ''
  if (!operatingSystem || process.arch !== 'x64') throw new Error('release binaries currently support only Windows x64 and Linux x64')
  return `${operatingSystem}-x64`
}

function validateReceipt(data) {
  let receipt
  try {
    receipt = JSON.parse(data)
  } catch (error) {
    throw new Error(`compatibility check did not return JSON: ${error.message}`)
  }
  if (receipt.schemaVersion !== 1 || !Array.isArray(receipt.imageExports) || !receipt.sprites || !receipt.materialSet || !Array.isArray(receipt.models) || !receipt.inspection) {
    throw new Error('compatibility check returned an incomplete receipt')
  }
  const imageFormats = new Set(receipt.imageExports.map(item => item.request?.format))
  for (const format of ['png', 'webp', 'avif']) if (!imageFormats.has(format)) throw new Error(`compatibility receipt omitted ${format} image processing`)
  if (!Array.isArray(receipt.materialSet.outputs) || receipt.materialSet.outputs.length < 3) throw new Error('compatibility receipt omitted loose KTX2 processing')
  if (receipt.models.length === 0) throw new Error('compatibility receipt omitted glTF optimization')
  const inspectedFormats = new Set((receipt.inspection.assets || []).map(asset => asset.format))
  for (const format of ['png', 'webp', 'avif', 'ktx2', 'gltf']) if (!inspectedFormats.has(format)) throw new Error(`compatibility receipt omitted ${format} inspection`)
}

function isolatedEnvironment(directory) {
  const environment = { ...process.env, PATH: '' }
  delete environment.Path
  delete environment.NODE_PATH
  delete environment.NODE_OPTIONS
  environment.LOCALAPPDATA = join(directory, 'cache')
  environment.XDG_CACHE_HOME = join(directory, 'cache')
  return environment
}

function checkStandaloneBinary(binary, executableName) {
  const directory = mkdtempSync(join(tmpdir(), 'fabricum-binary-check-'))
  const isolated = join(directory, executableName)
  try {
    cpSync(binary, isolated)
    if (process.platform !== 'win32') chmodSync(isolated, 0o755)
    const environment = isolatedEnvironment(directory)
    const result = spawnSync(isolated, ['compatibility-check'], { cwd: directory, env: environment, encoding: 'utf8', windowsHide: true })
    if (result.error) throw new Error(`standalone binary could not start: ${result.error.message}`)
    if (result.status !== 0) throw new Error(`standalone compatibility check failed: ${(result.stderr || '').trim()}`)
    validateReceipt(result.stdout)
    const notices = spawnSync(isolated, ['third-party-notices'], { cwd: directory, env: environment, encoding: 'utf8', windowsHide: true })
    if (notices.error || notices.status !== 0 || !notices.stdout.includes('THIRD-PARTY-NOTICES.md')) {
      throw new Error(`standalone binary did not expose its embedded notices: ${(notices.stderr || '').trim()}`)
    }
  } finally {
    rmSync(directory, { recursive: true, force: true })
  }
}

function cleanObsoleteOutputs(outputDirectory, output, executableName, platform) {
  const defaultOutputDirectory = resolve(root, 'bin')
  if (outputDirectory !== defaultOutputDirectory) return
  const releasePattern = platform === 'windows-x64'
    ? /^fabricum-.+-windows-x64\.exe(?:\.sha256)?$/
    : /^fabricum-.+-linux-x64(?:\.sha256)?$/
  for (const name of readdirSync(defaultOutputDirectory)) {
    const path = join(defaultOutputDirectory, name)
    if (path !== output && path !== `${output}.sha256` && (name === executableName || name === `${executableName}.sha256` || releasePattern.test(name))) {
      rmSync(path, { force: true })
    }
  }
  const legacyDirectory = resolve(root, 'bin/release')
  if (dirname(legacyDirectory) !== defaultOutputDirectory) throw new Error('refusing to clean an unsafe legacy release path')
  rmSync(legacyDirectory, { recursive: true, force: true })
}

function main() {
  const args = process.argv.slice(2)
  if (args.includes('--help')) {
    console.log('Usage: node scripts/release/binary.mjs [--platform linux-x64|windows-x64] [--output directory]')
    return
  }
  const version = readFileSync(join(root, 'VERSION'), 'utf8').trim()
  const platform = option(args, '--platform') || process.env.FABRICUM_RELEASE_PLATFORM || platformName()
  if (platform !== platformName()) throw new Error(`release platform ${platform} does not match this runner`)
  const executableName = process.platform === 'win32' ? 'fabricum.exe' : 'fabricum'
  const nativeDefault = process.env.RUNNER_TEMP ? join(process.env.RUNNER_TEMP, 'fabricum-codecs') : 'bin/codecs'
  const nativeDirectory = resolve(root, process.env.FABRICUM_NATIVE_OUTPUT_DIR || nativeDefault)
  const defaultOutputDirectory = resolve(root, 'bin')
  const outputDirectory = resolve(root, option(args, '--output') || process.env.FABRICUM_RELEASE_OUTPUT_DIR || defaultOutputDirectory)
  const releaseName = `fabricum-${version}-${platform}${process.platform === 'win32' ? '.exe' : ''}`
  const output = join(outputDirectory, releaseName)
  const workspace = mkdtempSync(join(tmpdir(), 'fabricum-release-build-'))
  const input = join(workspace, executableName)
  mkdirSync(outputDirectory, { recursive: true })
  try {
    run(process.execPath, ['scripts/build.mjs', '--output', input], { cwd: root })
    run('go', ['run', './back-end/cmd/fabricum-pack', '--input', input, '--native-directory', nativeDirectory, '--output', output, '--root', root, '--platform', platform], { cwd: root })
    checkStandaloneBinary(output, basename(output))
    const bytes = statSync(output).size
    if ((bytes < expectedRange.minimum || bytes > expectedRange.maximum) && !process.env.FABRICUM_SIZE_EXPLANATION) {
      throw new Error(`standalone binary size ${(bytes / 1024 / 1024).toFixed(2)} MiB is outside the expected 15-50 MiB range`)
    }
    const hash = createHash('sha256').update(readFileSync(output)).digest('hex')
    writeFileSync(`${output}.sha256`, `${hash}  ${releaseName}\n`)
    cleanObsoleteOutputs(outputDirectory, output, executableName, platform)
    console.log(`Created ${output}`)
    console.log(`Standalone binary size: ${(bytes / 1024 / 1024).toFixed(2)} MiB (${bytes} bytes)`)
  } finally {
    rmSync(workspace, { recursive: true, force: true })
  }
}

try {
  main()
} catch (error) {
  console.error(`Release binary failed: ${error.message}`)
  process.exitCode = 1
}
