import { cp, chmod } from 'node:fs/promises'
import { join } from 'node:path'

// Only the static libjpeg CLI is needed. SIMD is irrelevant to coefficient
// transcoding, so disabling it avoids a NASM build dependency on both platforms.
export async function installJPEGOptimizer({ pinned, workspace, outputDirectory, download, verify, extract, run, findFile, platform = process.platform, arch = process.arch }) {
  if (arch !== 'x64' || !['win32', 'linux'].includes(platform)) throw new Error('pinned jpegtran requires Windows or Linux x64')
  const archive = join(workspace, 'libjpeg-turbo.tar.gz')
  const directory = join(workspace, 'libjpeg-turbo')
  await download(pinned.source, archive)
  await verify(archive, pinned.sourceSha256)
  await extract(archive, directory)
  const source = join(directory, `libjpeg-turbo-${pinned.version}`)
  const build = join(workspace, 'jpegtran-build')
  run('cmake', ['-S', source, '-B', build, '-DCMAKE_BUILD_TYPE=Release', '-DENABLE_SHARED=OFF', '-DENABLE_STATIC=ON', '-DWITH_TURBOJPEG=OFF', '-DWITH_SIMD=OFF', '-DWITH_CRT_DLL=OFF'])
  run('cmake', ['--build', build, '--config', 'Release', '--target', 'jpegtran-static', '--parallel', '2'])
  const suffix = platform === 'win32' ? '.exe' : ''
  const executable = await findFile(build, `jpegtran-static${suffix}`)
  if (!executable) throw new Error('native JPEG build did not produce jpegtran-static')
  const destination = join(outputDirectory, `jpegtran${suffix}`)
  await cp(executable, destination)
  if (platform !== 'win32') await chmod(destination, 0o755)
}
