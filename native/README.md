# Native codec tools

WebP and AVIF encoding uses `cwebp` and `avifenc` directly. Put both
executables in `PATH`, or point Fabricum's `encoderDirectory` setting (or
`--encoder-directory`) at the directory containing them.

The pinned inputs are in [`versions.json`](versions.json). The selected AVIF
encoder is libaom 3.14.1, selected explicitly with `avifenc --codec aom`.

## Linux

Use the official libwebp 1.6.0 Linux x64 archive for `cwebp` and the official
libavif 1.4.2 `linux-artifacts.zip` for `avifenc` and `avifdec`. The artifact
is built with the pinned local libaom 3.14.1 encoder. Verify the digests in
`versions.json` before putting the executables in `PATH`.

## Windows

Use the official libwebp 1.6.0 x64 archive and the libavif 1.4.2
`windows-artifacts.zip`, verifying the digests in `versions.json`. The libavif
artifact is built with libaom; Fabricum still passes `--codec aom` so a
different codec cannot be selected accidentally. The command-line phase
intentionally leaves static CGO bindings for a later slice.
