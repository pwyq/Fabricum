import { join } from 'node:path'

// Linux uses upstream's static musl binary. Upstream's Windows MSVC binary
// imports VCRUNTIME140.dll, so build with GNU and a static runtime instead.
export async function installPNGOptimizer({ pinned, workspace, download, verify, extract, run, copyExecutables, platform = process.platform, arch = process.arch }) {
  if (arch !== 'x64') throw new Error('pinned oxipng tools require x64')
  const archive = join(workspace, 'oxipng.tar.gz')
  const directory = join(workspace, 'oxipng')
  if (platform === 'win32') {
    await download(pinned.source, archive)
    await verify(archive, pinned.sourceSha256)
    await extract(archive, directory)
    const source = join(directory, `oxipng-${pinned.version}`)
    const environment = { ...process.env, RUSTUP_TOOLCHAIN: pinned.windows.toolchain, RUSTFLAGS: '-C target-feature=+crt-static' }
    // This variable takes precedence over RUSTFLAGS; prevent host settings
    // from silently removing static runtime linkage.
    delete environment.CARGO_ENCODED_RUSTFLAGS
    run('cargo', ['build', '--locked', '--release', '--target', pinned.windows.target], source, environment)
    await copyExecutables(workspace, [join(source, 'target', pinned.windows.target, 'release')], false, ['oxipng'])
  } else if (platform === 'linux') {
    await download(pinned.linux.artifact, archive)
    await verify(archive, pinned.linux.sha256)
    await extract(archive, directory)
    await copyExecutables(workspace, [directory], false, ['oxipng'])
  } else {
    throw new Error(`unsupported PNG optimizer platform ${platform}`)
  }
}
