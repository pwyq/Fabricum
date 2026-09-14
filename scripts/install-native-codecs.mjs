import { createHash } from 'node:crypto'
import { chmod, cp, mkdtemp, mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const versions = JSON.parse(await readFile(join(root, 'native', 'versions.json'), 'utf8'))
const outputDirectory = process.env.FABRICUM_NATIVE_OUTPUT_DIR
  ? resolve(root, process.env.FABRICUM_NATIVE_OUTPUT_DIR)
  : process.env.RUNNER_TEMP
  ? join(process.env.RUNNER_TEMP, 'fabricum-codecs')
  : join(root, 'bin', 'codecs')

function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit', windowsHide: true })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} failed with status ${result.status}`)
}

function cmakeRuntimeOptions() {
  return process.platform === 'win32' ? ['-DCMAKE_MSVC_RUNTIME_LIBRARY=MultiThreaded'] : []
}

function errorDetail(error) {
  const detail = error instanceof Error ? error.message : String(error)
  const cause = error && typeof error === 'object' && error.cause instanceof Error ? ` (${error.cause.message})` : ''
  return `${detail}${cause}`
}

async function download(url, path) {
  const attempts = 3
  let lastError
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    let response
    try {
      response = await fetch(url)
    } catch (error) {
      lastError = error
    }
    if (response && !response.ok) throw new Error(`download failed (${response.status}): ${url}`)
    if (response) {
      try {
        await writeFile(path, Buffer.from(await response.arrayBuffer()))
        return
      } catch (error) {
        lastError = error
      }
    }
    if (attempt < attempts) {
      const delay = attempt * 1000
      console.warn(`Download attempt ${attempt}/${attempts} failed for ${url}: ${errorDetail(lastError)}; retrying in ${delay}ms`)
      await new Promise(resolve => setTimeout(resolve, delay))
    }
  }
  throw new Error(`download failed for ${url} after ${attempts} attempts: ${errorDetail(lastError)}`, { cause: lastError })
}

async function verify(path, expected) {
  const actual = createHash('sha256').update(await readFile(path)).digest('hex')
  if (actual !== expected) throw new Error(`SHA-256 mismatch for ${path}: expected ${expected}, got ${actual}`)
}

async function extract(archive, destination) {
  await mkdir(destination, { recursive: true })
  if (archive.toLowerCase().endsWith('.zip') && process.platform !== 'win32') {
    run('unzip', ['-q', archive, '-d', destination])
    return
  }
  const tar = process.platform === 'win32'
    ? join(process.env.SystemRoot || process.env.WINDIR || 'C:\\Windows', 'System32', 'tar.exe')
    : 'tar'
  run(tar, ['-xf', archive, '-C', destination])
}

async function findFile(directory, filename) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) {
      const found = await findFile(path, filename)
      if (found) return found
    } else if (entry.name.toLowerCase() === filename.toLowerCase()) {
      return path
    }
  }
  return ''
}

async function installWindows(workspace) {
  const webpArchive = join(workspace, 'libwebp.zip')
  const avifArchive = join(workspace, 'libavif.zip')
  await download(versions.webp.windows.artifact, webpArchive)
  await verify(webpArchive, versions.webp.windows.sha256)
  await download(versions.avif.windows.artifact, avifArchive)
  await verify(avifArchive, versions.avif.windows.sha256)
  await extract(webpArchive, join(workspace, 'webp'))
  await extract(avifArchive, join(workspace, 'avif'))
  await copyExecutables(workspace, [join(workspace, 'webp'), join(workspace, 'avif')], true)
  await copyRuntimeLibraries([join(workspace, 'webp'), join(workspace, 'avif')])
}

async function installLinux(workspace) {
  const webpArchive = join(workspace, 'libwebp.tar.gz')
  const avifArchive = join(workspace, 'libavif.zip')
  await download(versions.webp.linux.artifact, webpArchive)
  await verify(webpArchive, versions.webp.linux.sha256)
  await download(versions.avif.linux.artifact, avifArchive)
  await verify(avifArchive, versions.avif.linux.sha256)
  await extract(webpArchive, join(workspace, 'webp'))
  await extract(avifArchive, join(workspace, 'avif'))
  await copyExecutables(workspace, [join(workspace, 'webp'), join(workspace, 'avif')], true)
  await copyRuntimeLibraries([join(workspace, 'webp'), join(workspace, 'avif')])
}

async function installBasis(workspace) {
  const archive = join(workspace, 'basis_universal.tar.gz')
  await download(versions.basisu.source, archive)
  await verify(archive, versions.basisu.sha256)
  const sourceDirectory = join(workspace, 'basis')
  await extract(archive, sourceDirectory)
  const source = join(sourceDirectory, 'basis_universal-2_0_3')
  const build = join(workspace, 'basis-build')
  run('cmake', ['-S', source, '-B', build, `-DCMAKE_BUILD_TYPE=${versions.basisu.build.type}`, ...cmakeRuntimeOptions()])
  run('cmake', ['--build', build, '--config', versions.basisu.build.type, '--target', versions.basisu.build.target, '--parallel', '2'])
  await copyExecutables(workspace, [build, source], false, ['basisu'])
  await copyRuntimeLibraries([build])
}

async function installGltfpack(workspace) {
  const meshoptimizerArchive = join(workspace, 'meshoptimizer.tar.gz')
  const webpSourceArchive = join(workspace, 'libwebp-source.tar.gz')
  await download(versions.meshoptimizer.source, meshoptimizerArchive)
  await verify(meshoptimizerArchive, versions.meshoptimizer.sha256)
  await download(versions.webp.source, webpSourceArchive)
  await verify(webpSourceArchive, versions.webp.sourceSha256)
  const meshoptimizerRoot = join(workspace, 'meshoptimizer')
  const webpRoot = join(workspace, 'libwebp-source')
  await extract(meshoptimizerArchive, meshoptimizerRoot)
  await extract(webpSourceArchive, webpRoot)
  const meshoptimizerSource = join(meshoptimizerRoot, `meshoptimizer-${versions.meshoptimizer.version}`)
  const webpSource = join(webpRoot, `libwebp-${versions.webp.version}`)
  const basisSource = join(workspace, 'basis', 'basis_universal-2_0_3')
  const build = join(workspace, 'gltfpack-build')
  run('cmake', [
    '-S', meshoptimizerSource,
    '-B', build,
    '-DMESHOPT_BUILD_GLTFPACK=ON',
    '-DMESHOPT_BUILD_SHARED_LIBS=OFF',
    '-DMESHOPT_INSTALL=OFF',
    `-DMESHOPT_GLTFPACK_BASISU_PATH=${basisSource}`,
    `-DMESHOPT_GLTFPACK_LIBWEBP_PATH=${webpSource}`,
    `-DCMAKE_BUILD_TYPE=${versions.meshoptimizer.build.type}`,
    ...cmakeRuntimeOptions(),
  ])
  run('cmake', ['--build', build, '--config', versions.meshoptimizer.build.type, '--target', versions.meshoptimizer.build.target, '--parallel', '2'])
  await copyExecutables(workspace, [build], false, ['gltfpack'])
  await copyRuntimeLibraries([build])
}

async function copyExecutables(workspace, searchDirectories, includeDecoders = false, additional = []) {
  const filenames = [
    ...(includeDecoders
      ? (process.platform === 'win32'
        ? ['cwebp.exe', 'dwebp.exe', 'avifenc.exe', 'avifdec.exe']
        : ['cwebp', 'dwebp', 'avifenc', 'avifdec'])
      : []),
    ...additional.map(filename => process.platform === 'win32' ? `${filename}.exe` : filename),
  ]
  for (const filename of filenames) {
    let source = ''
    for (const directory of searchDirectories) {
      source = await findFile(directory, filename)
      if (source) break
    }
    if (!source) throw new Error(`native codec build did not produce ${filename}`)
    const destination = join(outputDirectory, filename)
    await cp(source, destination, { dereference: true })
    if (process.platform !== 'win32') await chmod(destination, 0o755)
  }
}

async function copyRuntimeLibraries(searchDirectories) {
  for (const directory of searchDirectories) await copyRuntimeLibrariesFrom(directory)
}

async function copyRuntimeLibrariesFrom(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const source = join(directory, entry.name)
    if (entry.isDirectory()) {
      await copyRuntimeLibrariesFrom(source)
      continue
    }
    const isLibrary = process.platform === 'win32'
      ? entry.name.toLowerCase().endsWith('.dll')
      : /\.so(?:\.\d+)*$/.test(entry.name)
    if (isLibrary) await cp(source, join(outputDirectory, entry.name), { dereference: true })
  }
}

async function main() {
  if (process.platform !== 'win32' && process.platform !== 'linux') {
    throw new Error(`native codec setup is currently supported on Windows and Linux, not ${process.platform}`)
  }
  await mkdir(outputDirectory, { recursive: true })
  const workspace = await mkdtemp(join(tmpdir(), 'fabricum-native-'))
  if (process.platform === 'win32') await installWindows(workspace)
  else await installLinux(workspace)
  await installBasis(workspace)
  await installGltfpack(workspace)
  if (process.env.GITHUB_PATH) await writeFile(process.env.GITHUB_PATH, `${outputDirectory}\n`, { flag: 'a' })
  console.log(`Native codec tools installed in ${outputDirectory}`)
}

main().catch((error) => {
  console.error(`Native codec setup failed: ${error.message}`)
  process.exitCode = 1
})
